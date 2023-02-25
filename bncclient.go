package bncclient

import (
	"encoding/json"
	"errors"
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

type OneTrade struct {
	Id           int64   `json:"id"`
	Price        float64 `json:"price,string"`
	Qty          float64 `json:"qty,string"`
	QuoteQty     float64 `json:"quoteQty,string"`
	Time         int64   `json:"time"`
	IsBuyerMaker bool    `json:"isBuyerMaker"`
	IsBestMatch  bool    `json:"isBestMatch"`
}

type AggTrade struct {
	AggTradeId      int64   `json:"a"`
	AggPrice        float64 `json:"p,string"`
	AggQty          float64 `json:"q,string"`
	FirstTradeId    int64   `json:"f"`
	LastTradeId     int64   `json:"l"`
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

type TradesList []OneTrade
type AggTradesList []AggTrade

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

func (bc *BinanceClient) GetServerTime() (int64, Warning, error) {
	type ServerTimeIntermediateFormat struct {
		ServerTime int64 `json:"serverTime"`
	}

	var timestampTmp ServerTimeIntermediateFormat

	timestampRaw, warning, err := (*bc).makeApiRequest("/api/v3/time", bc.apiKey, map[string]string{}, 1)

	if err != nil {
		return 0, nil, err
	}

	if warning != nil {
		return 0, warning, nil
	}

	// Try to parse JSON and return error if it is:
	if err := bc.tryParseResponse(timestampRaw, &timestampTmp); err != nil {
		return 0, nil, err
	}

	return timestampTmp.ServerTime, nil, nil
}

// GetOrderBook - gets order book. Valid values for limit: [5, 10, 20, 50, 100, 500, 1000, 5000]
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#order-book
func (bc *BinanceClient) GetOrderBook(symbol string, limit int) (OrderBook, Warning, error) {
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

	orderBookRaw, warning, err := (*bc).makeApiRequest("/api/v3/depth", bc.apiKey, queryParams, limitToWeightMap[limit])

	if err != nil {
		return OrderBook{}, nil, err
	}

	if warning != nil {
		return OrderBook{}, warning, nil
	}

	// Try to parse JSON and return error if it is:
	if err := bc.tryParseResponse(orderBookRaw, &orderBookTmp); err != nil {
		return OrderBook{}, nil, err
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

	return orderBook, nil, nil
}

// GetRecentTrades - Get recent trades.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#recent-trades-list
// Parameter limit is optional, set it to -1 if you don't want to specify it.
func (bc *BinanceClient) GetRecentTrades(symbol string, limit int) (TradesList, Warning, error) {
	var recentTrades TradesList
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	recentTradesRaw, warning, err := (*bc).makeApiRequest("/api/v3/trades", bc.apiKey, queryParams, 1)

	if err != nil {
		return nil, nil, err
	}

	if warning != nil {
		return nil, warning, nil
	}

	if err := bc.tryParseResponse(recentTradesRaw, &recentTrades); err != nil {
		return nil, nil, err
	}

	return recentTrades, nil, nil
}

// GetHistoricalTrades - Get older trades.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#old-trade-lookup-market_data
// Parameters limit and fromId are optional, if you don't want to specify them, set them to -1
func (bc *BinanceClient) GetHistoricalTrades(symbol string, limit int, fromId int64) (TradesList, Warning, error) {
	var historicalTrades TradesList
	queryParams := make(map[string]string)
	queryParams["symbol"] = symbol

	if limit >= 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	if fromId >= 0 {
		queryParams["fromId"] = strconv.FormatInt(fromId, 10)
	}

	historicalTradesRaw, warning, err := (*bc).makeApiRequest("/api/v3/historicalTrades", bc.apiKey, queryParams, 5)

	if err != nil {
		return nil, nil, err
	}

	if warning != nil {
		return nil, warning, nil
	}

	if err := bc.tryParseResponse(historicalTradesRaw, &historicalTrades); err != nil {
		return nil, nil, err
	}

	return historicalTrades, nil, nil
}

// GetAggregatedTrades - Get compressed, aggregate trades. Trades that fill at the time, from the same taker order, with the same price will have the quantity aggregated.
// Details: https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#compressedaggregate-trades-list
// ATTENTION! If you don't want specify optional params - fromId, startTimeMS, endTimeMS, limit set it to -1 (not 0!)
// So sad that Go does not have default parameters!
func (bc *BinanceClient) GetAggregatedTrades(symbol string, fromId int64, startTimeMS int64, endTimeMS int64, limit int) (AggTradesList, Warning, error) {

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

	aggTradesRaw, warning, err := (*bc).makeApiRequest("/api/v3/aggTrades", bc.apiKey, queryParams, 1)

	if err != nil {
		return nil, nil, err
	}

	if warning != nil {
		return nil, warning, nil
	}

	if err := bc.tryParseResponse(aggTradesRaw, &aggTrades); err != nil {
		return nil, nil, err
	}

	return aggTrades, nil, nil
}

// makeApiRequest creates API request and performs it.
// Returns raw (not parsed) response (as slice of bytes), status code, recommended sleep time (ms) and error.
// path - is local path, like "/api/v3/trades",
// apiKey - is your unique API key (X-MBX-APIKEY header),
// queryParams is map with GET-parameters (map can be empty, if no GET parameters needed).
// Returned parameters:
// 1. Raw response (bytes)
// 2. Warning - when calling functionality should wait some time to ot spam the API
// 3. Error - when something went bad.
func (bc *BinanceClient) makeApiRequest(path string, apiKey string, queryParams map[string]string, weight int) ([]byte, Warning, error) {

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
		warning := newWaring(sleepTimeMS, fmt.Sprintf("Request limit reached. We should sleep %d sec to avoid abuse Binance API.\n", sleepTimeMS/1000))
		return nil, warning, nil
	}

	// ==================== THE CRITICAL POINT - REQUEST TO REMOTE API =================================================
	client := &http.Client{}
	request, err := http.NewRequest("GET", requestUrl.String(), nil)

	if err != nil {
		return nil, nil, err
	}

	request.Header.Set("X-MBX-APIKEY", apiKey)
	rawResponse, err := client.Do(request)

	// In this case error is not critical, usually it occurs because of network failure
	if err != nil {
		warning := newWaring(10*1000, "Temporary network problem. Try again later.")
		return nil, warning, nil
	}

	defer rawResponse.Body.Close()
	// =================================================================================================================

	if err != nil {
		// Maybe temporary network error (maybe print HTTP Code?):
		return nil, nil, err
	}

	bodyBytes, err := ioutil.ReadAll(rawResponse.Body)

	if err != nil {
		return nil, nil, err
	}

	switch true {
	case rawResponse.StatusCode == 403:
		// Most likely we have CloudFront error here, NOT a Binance error! So let's just wait a minute and try again.
		// TODO: Write RAW response to LOG file!
		fmt.Printf("Error 403 received. RAW error message: %s\n", string(bodyBytes))
		warning := newWaring(60*1000, fmt.Sprintf("Status Code 403 received. Usually it's CloudFront error.\n"))
		return nil, warning, nil

	case rawResponse.StatusCode == 429:
		retryAfter, _ := strconv.Atoi(rawResponse.Header.Get("Retry-After")) // seconds!
		// Receiving error 429 is a normal situation, so we don't want to put it out on the screen.
		//fmt.Printf("WARNING: Status Code 429 received. Binance API ask to wait %d seconds to avoid ban!\n", retryAfter)
		warning := newWaring(int64(retryAfter*1000), fmt.Sprintf("Status Code 429 received. Binance API ask to wait %d seconds to avoid ban!\n", retryAfter))
		return nil, warning, nil

	case rawResponse.StatusCode != 200:
		// TODO: Write RAW response to LOG file!
		fmt.Printf("UNKNOWN ERROR: Status Code %d received. RAW error message: %s\n", rawResponse.StatusCode, string(bodyBytes))
		return nil, nil, errors.New(fmt.Sprintf("UNKNOWN ERROR: Status Code %d received. RAW error message: %s\n", rawResponse.StatusCode, string(bodyBytes)))

	default:
		return bodyBytes, nil, nil
	}
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
