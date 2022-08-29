package bncclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

type BinanceClient struct {
	apiKey           string
	weightController *weightController
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

type binanceError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func NewBinanceClient(apiKey string) *BinanceClient {
	return &BinanceClient{
		apiKey:           apiKey,
		weightController: getWeightControllerSingleton(),
	}
}

func (bc *BinanceClient) GetServerTime() (int64, int64, error, error) {
	type ServerTimeIntermediateFormat struct {
		ServerTime int64 `json:"serverTime"`
	}

	var timestampTmp ServerTimeIntermediateFormat

	timestampRaw, retryAfter, warning, err := (*bc).makeApiRequest("/api/v3/time", bc.apiKey, map[string]string{}, 1)

	if err != nil {
		return 0, 0, nil, err
	}

	if warning != nil {
		return 0, retryAfter, warning, nil
	}

	// Try to parse JSON and return error if it is:
	error := bc.tryParseResponse(timestampRaw, &timestampTmp)

	return timestampTmp.ServerTime, retryAfter, nil, error
}

// GetOrderBook - gets order book. Valid values for limit: [5, 10, 20, 50, 100, 500, 1000, 5000]
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#order-book
func (bc *BinanceClient) GetOrderBook(symbol string, limit int) (OrderBook, int64, error, error) {
	limitToWeightMap := map[int]int{
		-1:   1,
		5:    1,
		10:   1,
		20:   1,
		50:   1,
		100:  1,
		500:  5,
		1000: 10,
		5000: 50,
	}

	if _, exists := limitToWeightMap[limit]; !exists {
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

	orderBookRaw, retryAfter, warning, err := (*bc).makeApiRequest("/api/v3/depth", bc.apiKey, queryParams, limitToWeightMap[limit])

	if err != nil {
		return OrderBook{}, 0, nil, err
	}

	if warning != nil {
		return OrderBook{}, retryAfter, warning, nil
	}

	// Try to parse JSON and return error if it is:
	if err := bc.tryParseResponse(orderBookRaw, &orderBookTmp); err != nil {
		return OrderBook{}, 0, nil, err
	}

	var orderBook OrderBook // The final version of order book, which we will return.
	orderBook.LastUpdateId = orderBookTmp.LastUpdateId

	orderBook.Bids = make([]struct {
		Price float64
		Qty   float64
	}, len(orderBookTmp.Bids)) // len(orderBookTmp.Bids) is almost the same as "limit", but we can't rely on limit because it is optional parameter.

	orderBook.Asks = make([]struct {
		Price float64
		Qty   float64
	}, len(orderBookTmp.Asks)) // len(orderBookTmp.Asks) is almost the same as "limit", but we can't rely on limit because it is optional parameter.

	for i := 0; i < len(orderBookTmp.Bids); i++ {
		orderBook.Bids[i].Price, _ = orderBookTmp.Bids[i][0].Float64()
		orderBook.Bids[i].Qty, _ = orderBookTmp.Bids[i][1].Float64()
	}

	for i := 0; i < len(orderBookTmp.Asks); i++ {
		orderBook.Asks[i].Price, _ = orderBookTmp.Asks[i][0].Float64()
		orderBook.Asks[i].Qty, _ = orderBookTmp.Asks[i][1].Float64()
	}

	return orderBook, retryAfter, nil, nil
}

// GetRecentTrades - Get recent trades.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#recent-trades-list
// Parameter limit is optional, set it to -1 if you don't want to specify it.
func (bc *BinanceClient) GetRecentTrades(symbol string, limit int) (TradesList, int64, error, error) {
	var recentTrades TradesList
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	recentTradesRaw, retryAfter, warning, err := (*bc).makeApiRequest("/api/v3/trades", bc.apiKey, queryParams, 1)

	if err != nil {
		return nil, 0, nil, err
	}

	if warning != nil {
		return nil, retryAfter, warning, nil
	}

	error := bc.tryParseResponse(recentTradesRaw, &recentTrades)

	return recentTrades, retryAfter, nil, error
}

// GetHistoricalTrades - Get older trades.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#old-trade-lookup-market_data
// Parameters limit and fromId are optional, if you don't want to specify them, set them to -1
func (bc *BinanceClient) GetHistoricalTrades(symbol string, limit int, fromId int64) (TradesList, int64, error, error) {
	var historicalTrades TradesList
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	if fromId >= 0 {
		queryParams["fromId"] = strconv.FormatInt(fromId, 10)
	}

	historicalTradesRaw, retryAfter, warning, err := (*bc).makeApiRequest("/api/v3/historicalTrades", bc.apiKey, queryParams, 5)

	if err != nil {
		return nil, 0, nil, err
	}

	if warning != nil {
		return nil, retryAfter, warning, nil
	}

	error := bc.tryParseResponse(historicalTradesRaw, &historicalTrades)

	return historicalTrades, retryAfter, nil, error
}

// GetAggregatedTrades - Get compressed, aggregate trades. Trades that fill at the time, from the same taker order, with the same price will have the quantity aggregated.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#compressedaggregate-trades-list
// ATTENTION! If you don't want specify optional params - fromId, startTimeMS, endTimeMS, limit set it to -1 (not 0!)
// So sad that Go does not have default parameters!
func (bc *BinanceClient) GetAggregatedTrades(symbol string, fromId int64, startTimeMS int64, endTimeMS int64, limit int) (AggTradesList, int64, error, error) {

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

	aggTradesRaw, retryAfter, warning, err := (*bc).makeApiRequest("/api/v3/aggTrades", bc.apiKey, queryParams, 1)

	if err != nil {
		return nil, 0, nil, err
	}

	if warning != nil {
		return nil, retryAfter, warning, nil
	}

	error := bc.tryParseResponse(aggTradesRaw, &aggTrades)

	return aggTrades, retryAfter, nil, error
}

// makeApiRequest creates API request and performs it.
// Returns raw (not parsed) response (as slice of bytes), status code, recommended sleep time (ms) and error.
// path - is local path, like "/api/v3/trades",
// apiKey - is your unique API key (X-MBX-APIKEY header),
// queryParams is map with GET-parameters (map can be empty, if no GET parameters needed).
// Returned parameters:
// 1. Raw response (bytes)
// 2. Recommended sleep time (MS)
// 3. Warning - when calling functionality should wait some time to ot spam the API
// 4. Error - when something went bad.
func (bc *BinanceClient) makeApiRequest(path string, apiKey string, queryParams map[string]string, weight int) ([]byte, int64, error, error) {

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

	// !!!BEFORE!!! polling the API, check accumulated weight and recommended sleep time (if it is):
	sleepTimeMS := bc.weightController.getSleepTime(weight) // Should be called only once per function call, because it's atomic counter!
	if sleepTimeMS > 0 {
		warning := newWaring(fmt.Sprintf("Request limit reached. We should sleep %d sec to avoid abuse Binance API.\n", sleepTimeMS/1000))
		return nil, sleepTimeMS, warning, nil
	}

	// ==================== THE CRITICAL POINT - REQUEST TO REMOTE API =================================================
	client := &http.Client{}
	request, _ := http.NewRequest("GET", requestUrl.String(), nil)
	request.Header.Set("X-MBX-APIKEY", apiKey)
	rawResponse, err := client.Do(request)
	defer rawResponse.Body.Close()
	// =================================================================================================================

	if err != nil {
		// Maybe temporary network error (maybe print HTTP Code?):
		return nil, 0, nil, err
	}

	bodyBytes, err := ioutil.ReadAll(rawResponse.Body)

	if err != nil {
		return nil, 0, nil, err
	}

	retryAfter, _ := strconv.Atoi(rawResponse.Header.Get("Retry-After")) // seconds!
	retryAfterMS := int64(retryAfter * 1000)


	if rawResponse.StatusCode == 429 && retryAfterMS > 0 {
		fmt.Printf("WARNING: Status Code 429 received. Binance API ask to wait %d seconds to avoid ban!\n", retryAfter)
		warning := newWaring(fmt.Sprintf("Status Code 429 received. Binance API ask to wait %d seconds to avoid ban!\n", retryAfter))
		return nil, retryAfterMS, warning, nil
	}

	return bodyBytes, retryAfterMS, nil, nil
}

func (bc *BinanceClient) tryParseResponse(rawResponse []byte, pointerToTargetStructure interface{}) error {

	var binanceErr binanceError

	if err := json.Unmarshal(rawResponse, pointerToTargetStructure); err != nil { // FIRST PARSE ATTEMPT: parse response to AggTradesList type
		if json.Unmarshal(rawResponse, &binanceErr) != nil { // SECOND PARSE ATTEMPT: parse to binanceError type
			return err // Parse to binanceError failed, so just return original error
		}
		return binanceErr
	}

	return nil
}

func (e binanceError) Error() string {
	return fmt.Sprintf("An error occured while requesting Binance API. Error code: %d, Native Binance message: %s", e.Code, e.Msg)
}

func (e binanceError) GetCode() int {
	return e.Code
}

func (e binanceError) GetMsg() string {
	return e.Msg
}
