package bncclient

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

type BinanceClient struct {
	apiKey string
}

type oneTrade struct {
	Id           int64   `json:"id"`
	Price        float64 `json:"price,string"`
	Qty          float64 `json:"qty,string"`
	QuoteQty     float64 `json:"quoteQty,string"`
	Time         int64   `json:"time"`
	IsBuyerMaker bool    `json:"isBuyerMaker"`
	IsBestMatch  bool    `json:"isBestMatch"`
}

type aggTrades struct {
	AggTradeId      int64   `json:"a"`
	AggPrice        float64 `json:"p,string"`
	AggQty          float64 `json:"q,string"`
	AggFirstTradeId int64   `json:"f"`
	AggLastTradeId  int64   `json:"l"`
	AggTime         int64   `json:"T"`
	AggIsBuyerMaker bool    `json:"m"`
	AggIsBestMatch  bool    `json:"M"`
}

type OrderBook struct {
	LastUpdateId int64
	Bids         []struct {
		Price float64
		Qty   float64
	}
	Asks []struct {
		Price float64
		Qty   float64
	}
}

type TradesList []oneTrade
type AggTradesList []aggTrades

func NewBinanceClient(apiKey string) *BinanceClient {
	return &BinanceClient{
		apiKey: apiKey,
	}
}

func (bc *BinanceClient) GetServerTime() (int64, int, int, error) {
	type ServerTimeIntermediateFormat struct {
		ServerTime int64 `json:"serverTime"`
	}

	var timestampTmp ServerTimeIntermediateFormat

	timestampRaw, statusCode, retryAfter, err := makeApiRequest("/api/v3/time", bc.apiKey, map[string]string{})

	if err != nil {
		return 0, 0, 0, err
	}

	// Try to parse JSON and return error if it is:
	if err := json.Unmarshal(timestampRaw, &timestampTmp); err != nil {
		return 0, 0, 0, err
	}

	return timestampTmp.ServerTime, statusCode, retryAfter, nil
}

// GetOrderBook - gets order book. Valid values for limit: [5, 10, 20, 50, 100, 500, 1000, 5000]
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#order-book
func (bc *BinanceClient) GetOrderBook(symbol string, limit int) (OrderBook, int, int, error) {
	inArray := func(value int, heap []int) bool {
		for i := 0; i < len(heap); i++ {
			if heap[i] == value {
				return true
			}
		}
		return false
	}

	if !inArray(limit, []int{-1, 5, 10, 20, 50, 100, 500, 1000, 5000}) {
		panic("Not allowed limit value!")
	}

	type OrderBookIntermediateFormat struct {
		LastUpdateId int64            `json:"lastUpdateId"`
		Bids         [][2]json.Number `json:"bids"`
		Asks         [][2]json.Number `json:"asks"`
	}

	var orderBookTmp OrderBookIntermediateFormat
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	orderBookRaw, statusCode, retryAfter, err := makeApiRequest("/api/v3/depth", bc.apiKey, queryParams)

	if err != nil {
		return OrderBook{}, 0, 0, err
	}

	// Try to parse JSON and return error if it is:
	if err := json.Unmarshal(orderBookRaw, &orderBookTmp); err != nil {
		return OrderBook{}, 0, 0, err
	}

	var orderBook OrderBook // The final version of order book, which we will return.
	orderBook.LastUpdateId = orderBookTmp.LastUpdateId

	orderBook.Bids = make([]struct {
		Price float64
		Qty   float64
	}, len(orderBookTmp.Bids)) // len(orderBookTmp.Bids) is almost the same as "limit", but we can't rely on limit because it is optional parameter.

	orderBook.Asks = make([]struct {
		Price float64
		Qty float64
	}, len(orderBookTmp.Asks)) // len(orderBookTmp.Asks) is almost the same as "limit", but we can't rely on limit because it is optional parameter.

	for i := 0; i < len(orderBookTmp.Bids); i++ {
		orderBook.Bids[i].Price, _ = orderBookTmp.Bids[i][0].Float64()
		orderBook.Bids[i].Qty, _ = orderBookTmp.Bids[i][1].Float64()
	}

	for i := 0; i < len(orderBookTmp.Asks); i++ {
		orderBook.Asks[i].Price, _ = orderBookTmp.Asks[i][0].Float64()
		orderBook.Asks[i].Qty, _ = orderBookTmp.Asks[i][1].Float64()
	}

	return orderBook, statusCode, retryAfter, nil
}

// GetRecentTrades - Get recent trades.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#recent-trades-list
// Parameter limit is optional, set it to -1 if you don't want to specify it.
func (bc *BinanceClient) GetRecentTrades(symbol string, limit int) (TradesList, int, int, error) {
	var recentTrades TradesList
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	recentTradesRaw, statusCode, retryAfter, err := makeApiRequest("/api/v3/trades", bc.apiKey, queryParams)

	if err != nil {
		return nil, 0, 0, err
	}

	// Try to parse JSON and return error if it is:
	if err := json.Unmarshal(recentTradesRaw, &recentTrades); err != nil {
		return nil, 0, 0, err
	}

	return recentTrades, statusCode, retryAfter, nil
}

// GetHistoricalTrades - Get older trades.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#old-trade-lookup-market_data
// Parameters limit and fromId are optional, if you don't want to specify them, set them to -1
func (bc *BinanceClient) GetHistoricalTrades(symbol string, limit int, fromId int64) (TradesList, int, int, error) {
	var historicalTrades TradesList
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	if fromId >= 0 {
		queryParams["fromId"] = strconv.FormatInt(fromId, 10)
	}

	historicalTradesRaw, statusCode, retryAfter, err := makeApiRequest("/api/v3/historicalTrades", bc.apiKey, queryParams)

	if err != nil {
		return nil, 0, 0, err
	}

	if err := json.Unmarshal(historicalTradesRaw, &historicalTrades); err != nil {
		return nil, 0, 0, err
	}

	return historicalTrades, statusCode, retryAfter, nil
}

// GetAggregatedTrades - Get compressed, aggregate trades. Trades that fill at the time, from the same taker order, with the same price will have the quantity aggregated.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#compressedaggregate-trades-list
// ATTENTION! If you don't want specify optional params - fromId, startTimeMS, endTimeMS, limit set it to -1 (not 0!)
// So sad that Go does not have default parameters!
func (bc *BinanceClient) GetAggregatedTrades(symbol string, fromId int64, startTimeMS int64, endTimeMS int64, limit int) (AggTradesList, int, int, error) {
	var aggTrades AggTradesList
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if startTimeMS >= 0 {
		queryParams["startTime"] = strconv.FormatInt(startTimeMS, 10)
	}

	if endTimeMS >= 0 {
		queryParams["endTime"] = strconv.FormatInt(endTimeMS, 10)
	}

	if fromId >= 0 {
		queryParams["fromId"] = strconv.FormatInt(fromId, 10)
	}

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	aggTradesRaw, statusCode, retryAfter, err := makeApiRequest("/api/v3/aggTrades", bc.apiKey, queryParams)

	if err != nil {
		return nil, 0, 0, err
	}

	if err := json.Unmarshal(aggTradesRaw, &aggTrades); err != nil {
		return nil, 0, 0, err
	}

	return aggTrades, statusCode, retryAfter, nil
}

// Creates API request and performs it.
// Returns raw (not parsed) response (as slice of bytes), status code, retry-after time and error.
// path - is local path, like "/api/v3/trades",
// apiKey - is your unique API key (X-MBX-APIKEY header),
// queryParams is map with GET-parameters (map can be empty, if no GET parameters needed).
func makeApiRequest(path string, apiKey string, queryParams map[string]string) ([]byte, int, int, error) {
	requestUrl := url.URL{}
	requestUrl.Scheme = "https"
	requestUrl.Host = "api.binance.com"
	requestUrl.Path = path

	if len(queryParams) > 0 {
		query := requestUrl.Query()
		for key, value := range queryParams {
			query.Set(key, value)
		}
		requestUrl.RawQuery = query.Encode()
	}

	client := &http.Client{}
	request, _ := http.NewRequest("GET", requestUrl.String(), nil)

	request.Header.Set("X-MBX-APIKEY", apiKey)
	rawResponse, err := client.Do(request)

	if err != nil {
		// Maybe temporary network error:
		return nil, 0, 0, err
	}

	defer rawResponse.Body.Close()

	bodyBytes, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, 0, 0, err
	}

	retryAfter, _ := strconv.Atoi(rawResponse.Header.Get("Retry-After"))

	return bodyBytes, rawResponse.StatusCode, retryAfter, nil
}
