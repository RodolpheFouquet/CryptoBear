package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/olekukonko/tablewriter"
)

const APIKey = "94106f39d8683a731aa6a5a6cdac4016be9206378e3178ada28af24c457e23f8"

type CryptoCompareDate time.Time
type CryptoCompareLiveTimestamp time.Time

// MarketPair bblah
type MarketPair struct {
	Exchange     string            `json:"exchange"`
	ExchangeFSym string            `json:"exchange_fsym"`
	ExchangeTSym string            `json:"exchange_tsym"`
	FSym         string            `json:"fsym"`
	TSym         string            `json:"tsym"`
	LastUpdate   CryptoCompareDate `json:"last_update"`
}

// ToSubscriptionString translates a market pair to a websocket subscription string
func (pair MarketPair) ToSubscriptionString() string {
	return fmt.Sprintf("8~%v~%v~%v", pair.Exchange, pair.ExchangeFSym, pair.ExchangeTSym)
}

// UnmarshalJSON to trnaslate a timestamp to a time.Time
// TODO: parse nanoseconds
func (p *CryptoCompareDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	comps := strings.Split(s, ".")
	timestamp, err := strconv.ParseInt(comps[0], 10, 64)

	if err != nil {
		return err
	}

	t := time.Unix(timestamp, 0)
	*p = CryptoCompareDate(t)
	return nil
}

// Time transforms CryptoCompareDate to a time.Time type
func (p CryptoCompareDate) Time() time.Time {
	return time.Time(p)
}

// MarketPairResponseData represents the content of the Data structure
type MarketPairResponseData struct {
	Current    []MarketPair `json:"current"`
	Historical []MarketPair `json:"historical"`
}

// MarketPairResponse represents the data structure received from the api
type MarketPairResponse struct {
	Response   string                 `json:"Response"`
	Message    string                 `json:"Message"`
	HasWarning bool                   `json:"HasWarning"`
	Type       int                    `json:"Type"`
	RateLimit  json.RawMessage        `json:"RateLimit"` // not documented?
	Data       MarketPairResponseData `json:"Data"`
}

// Subscription message to be sent via websockets
type Subscription struct {
	Action        string   `json:"action"`
	Subscriptions []string `json:"subs"`
}

// LiveMessage blah
type LiveMessage struct {
	Type string `json:"TYPE"`
	Data json.RawMessage
}

// Time transforms CryptoCompareLiveTimestamp to a time.Time type
func (p CryptoCompareLiveTimestamp) Time() time.Time {
	return time.Time(p)
}

// UnmarshalJSON to trnaslate a timestamp to a time.Time
// TODO: parse nanoseconds
func (p *CryptoCompareLiveTimestamp) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	timestamp, err := strconv.ParseInt(s, 10, 64)

	if err != nil {
		return err
	}

	t := time.Unix(timestamp/1000000000, int64(math.Remainder(float64(timestamp), 1000000000)))
	*p = CryptoCompareLiveTimestamp(t)
	return nil
}

type OrderbookMessage struct {
	Type       string                     `json:"TYPE"`
	M          string                     `json:"M"`
	FSym       string                     `json:"FSYM"`
	TSym       string                     `json:"TSYM"`
	Side       int                        `json:"SIDE"`
	Action     int                        `json:"ACTION"`
	CCSEQ      int                        `json:"CCSEQ"`
	P          float64                    `json:"P"`
	Q          float64                    `json:"Q"`
	SEQ        int                        `json:"SEQ"`
	REPORTEDNS CryptoCompareLiveTimestamp `json:"REPORTEDNS"`
	DELAYNS    CryptoCompareLiveTimestamp `json:"DELAYNS"`
}

type Positions map[float64]float64

type Report struct {
	BID10            Positions
	ASK10            Positions
	LastBidPositions Positions
	LastAskPositions Positions
	Title            string
}

// Print blah
func (r Report) Print() {
	keys := make([]float64, 0, len(r.BID10))
	for k := range r.BID10 {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	table := tablewriter.NewWriter(os.Stdout)
	bidHeader := []string{r.Title}
	bidLine := []string{"BIDs"}
	meanBid := 0.0
	totalVid := 0.0

	for _, v := range keys {
		bidHeader = append(bidHeader, fmt.Sprintf("%.10f", v))
		bidLine = append(bidLine, fmt.Sprintf("%f", r.BID10[v]))
		meanBid += v * r.BID10[v]
		totalVid += r.BID10[v]
	}
	meanBid = meanBid / totalVid
	bidHeader = append(bidHeader, "Mean")
	bidLine = append(bidLine, fmt.Sprintf("%f", meanBid))
	table.SetHeader(bidHeader)
	table.Append(bidLine)
	table.Render()

	keys = make([]float64, 0, len(r.ASK10))
	meanAsk := 0.0
	totalAsked := 0.0
	for k := range r.ASK10 {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	table2 := tablewriter.NewWriter(os.Stdout)
	askHeader := []string{r.Title}
	askLine := []string{"ASks"}

	for _, v := range keys {
		askHeader = append(askHeader, fmt.Sprintf("%.10f", v))
		askLine = append(askLine, fmt.Sprintf("%f", r.ASK10[v]))
		meanAsk += v * r.ASK10[v]
		totalAsked += r.ASK10[v]
	}
	meanAsk = meanAsk / totalAsked
	askHeader = append(askHeader, "Mean")
	askLine = append(askLine, fmt.Sprintf("%f", meanAsk))
	table2.SetHeader(askHeader)
	table2.Append(askLine)
	table2.Render()

	log.Printf("The current mid price for %v is %.10f\n", r.Title, (meanAsk+meanBid)/2.0)
}

// InsertAndKeep10 blah
func (p Positions) InsertAndKeep10(book OrderbookMessage) {
	if v, ok := p[book.P]; ok {
		p[book.P] = v + book.Q
	} else {
		p[book.P] = book.Q
	}

	keys := make([]float64, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	if len(keys) <= 10 {
		return
	}
	sort.Float64s(keys)
	delete(p, keys[0])
}

// GetMarketPairs Gets market pair from the api
func GetMarketPairs() (pairs []MarketPair, err error) {

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://min-api.cryptocompare.com/data/v2/pair/mapping/exchange?e=Binance", nil)

	if err != nil {
		return pairs, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Apikey %v", APIKey))
	res, err := client.Do(req)
	if err != nil {
		return pairs, err
	}
	if res.StatusCode != http.StatusOK {
		return pairs, err
	}

	responseObject := &MarketPairResponse{}
	err = json.NewDecoder(res.Body).Decode(responseObject)
	if err != nil {
		return pairs, err
	}

	pairs = responseObject.Data.Current
	return pairs, nil
}

func main() {
	var protectLastHeartbeat = &sync.Mutex{}
	lastReceived := time.Now()

	pairs, err := GetMarketPairs()
	if err != nil {
		log.Fatalf(err.Error())
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	randomSource := rand.NewSource(time.Now().UnixNano())
	randomGeneration := rand.New(randomSource)
	p1 := pairs[randomGeneration.Intn(len(pairs))]
	p2 := pairs[randomGeneration.Intn(len(pairs))]

	subs := Subscription{Action: "SubAdd", Subscriptions: []string{p1.ToSubscriptionString(), p2.ToSubscriptionString()}}
	subData, err := json.Marshal(subs)
	if err != nil {
		log.Fatalf(err.Error())
	}

	endpoint := url.URL{Scheme: "wss", Host: "streamer.cryptocompare.com", Path: "/v2", RawQuery: fmt.Sprintf("api_key=%v", APIKey)}

	connection, _, err := websocket.DefaultDialer.Dial(endpoint.String(), nil)
	if err != nil {
		log.Fatalf(err.Error())
	}
	log.Println("Websocket connection established")
	defer connection.Close()

	reportsFirstPair := make(chan OrderbookMessage)
	reportsSecondPair := make(chan OrderbookMessage)
	go func() {
		report1 := Report{BID10: make(Positions), ASK10: make(Positions), Title: fmt.Sprintf("%v -> %v", p1.ExchangeFSym, p1.ExchangeTSym)}
		report2 := Report{BID10: make(Positions), ASK10: make(Positions), Title: fmt.Sprintf("%v -> %v", p2.ExchangeFSym, p2.ExchangeTSym)}
		defer close(reportsFirstPair)
		defer close(reportsSecondPair)
		var reportTicker *time.Ticker
		if time.Now().Second() < 15 {
			reportTicker = time.NewTicker(time.Second * time.Duration(15-time.Now().Second()))
		} else if time.Now().Second() < 30 {
			reportTicker = time.NewTicker(time.Second * time.Duration(30-time.Now().Second()))

		} else if time.Now().Second() < 45 {
			reportTicker = time.NewTicker(time.Second * time.Duration(45-time.Now().Second()))

		} else {
			reportTicker = time.NewTicker(time.Second * time.Duration(60-time.Now().Second()))

		}
		defer reportTicker.Stop()

		for {
			select {
			case <-interrupt:
				log.Println("interrupt")
				return
			case book := <-reportsFirstPair:
				{
					if book.Side == 0 {
						report1.BID10.InsertAndKeep10(book)
					} else {
						report1.ASK10.InsertAndKeep10(book)
					}
				}
				break
			case book2 := <-reportsFirstPair:
				{
					if book2.Side == 0 {
						report2.BID10.InsertAndKeep10(book2)
					} else {
						report2.ASK10.InsertAndKeep10(book2)
					}
				}
				break
			case <-reportTicker.C:
				reportTicker = time.NewTicker(15 * time.Second)
				report1.Print()

			}

		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := connection.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			messageData := LiveMessage{}
			err = json.Unmarshal(message, &messageData)
			if err != nil {
				log.Printf("Error while decoding message %v", err)
				return
			}
			log.Printf("recv: %s", message)
			switch messageData.Type {
			case "999":
				protectLastHeartbeat.Lock()
				lastReceived = time.Now()
				protectLastHeartbeat.Unlock()
				log.Println("Received Heartbeat")
				break
			case "20":
				log.Println("Received welcome, subscribing to the data channel")
				err = connection.WriteMessage(websocket.BinaryMessage, subData)
				if err != nil {
					log.Fatalf(err.Error())
					done <- struct{}{}
				}
				break
			case "8":
				book := OrderbookMessage{}
				err = json.Unmarshal(message, &book)
				if err != nil {
					log.Printf("Error while decoding message %v", err)
					return
				}
				if book.FSym == p1.ExchangeFSym && book.TSym == p1.ExchangeTSym {
					reportsFirstPair <- book
				}
				if book.FSym == p2.ExchangeFSym && book.TSym == p2.ExchangeTSym {
					reportsSecondPair <- book
				}
				break
			default:
				log.Println(fmt.Sprintf("Received other message %v", messageData.Type))
				break
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	checkHeartbeat := time.NewTicker(time.Minute)
	defer checkHeartbeat.Stop()

	for {
		select {
		case <-done:
			return
		case <-checkHeartbeat.C:
			func() { // do not forget to unlock, no matter what
				protectLastHeartbeat.Lock()
				defer protectLastHeartbeat.Lock()
				log.Println("Checking heartbeat")
				now := time.Now()
				if now.Sub(lastReceived).Minutes() >= 2 {
					log.Println("Heartbeat older than 2 minutes, the connection may have stalled")
					return
				}
			}()
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return

		}
	}
}
