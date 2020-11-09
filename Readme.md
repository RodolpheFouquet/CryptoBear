# Observations

The main difficulty was to find a reliable datasource for the exercice.
At the first time, after getting the pairs using the REST API, I sorted the pairs by last update to use the two most recently updated ones from this [call](https://min-api.cryptocompare.com/documentation?key=PairMapping&cat=pairMappingExchangeEndpoint).

It worked for some time, but at some point, I restarted the program, and I received no more message from the websocket, and so did the test APi in your documentation using the provided forms.

Then I decided to used two fixed pairs, maybe involving some well known currencies, like BTC -> ETH and BTC -> BAT.
It worked at first, but then again, stopped working while returning a 500 error, even in the browser.

So I ended up choosing two random pairs, and use trial and error to reliable streams of data.

Also, one observation the return object of this [call](https://min-api.cryptocompare.com/documentation?key=PairMapping&cat=pairMappingExchangeEndpoint) is not documented, you have to infer it from a test response, while the Order Book L2 socket message is well documented. However, websocket error messages, like 500, deserve a more complete documentation.

Finally, one little detail, the subject is not really clear about whether the datas should be cleared every 15 seconds or not, I made the educated guess that it had to, though I might be wrong.

I had to rush it a little bit in the end due to the timeframe, but it was fine, but I wish I had more time to do some unit tests.
