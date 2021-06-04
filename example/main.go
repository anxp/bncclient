package main

import (
	"bncclient"
	"fmt"
)

func main()  {
	client := bncclient.NewBinanceClient("PUT YOUR PUBLIC API KEY HERE")

	fmt.Println("======= AGGREGATED TRADES EXAMPLE OUTPUT ============================")
	limit := 4 // We'll get only 4 most recent aggregated trades

	aggTrades, statusCode, retryAfter, err := client.GetAggregatedTrades("ETHUSDT", -1, -1, -1, limit)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for i, tradeRecord := range aggTrades {
		fmt.Printf("Record#%d: %+v\n", i, tradeRecord)
	}

	fmt.Println("Status Code: ", statusCode)
	fmt.Println("Retry After: ", retryAfter)
	fmt.Println("=====================================================================")

	fmt.Println("======= ORDER BOOK EXAMPLE OUTPUT ===================================")
	orderBook, statusCode, retryAfter, err := client.GetOrderBook("ETHUSDT", 5)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("LastUpdateId: ", orderBook.LastUpdateId)

	for i, bidOrder := range orderBook.Bids {
		fmt.Printf("Bid order#%d: %+v\n", i, bidOrder)
	}

	for i, askOrder := range orderBook.Asks {
		fmt.Printf("Ask order#%d: %+v\n", i, askOrder)
	}

	fmt.Println("Status Code: ", statusCode)
	fmt.Println("Retry After: ", retryAfter)

	fmt.Println("=====================================================================")
}
