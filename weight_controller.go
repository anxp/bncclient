package bncclient

import (
	"sync"
	"time"
)

const weightLimitPerMinute = 1200 // Current Binance weight limit per minute is 1200
const sessionDurationMS = 60 * 1000

// weightController -- "weight counter" which accumulates total weight of requests and stops polling API when weight limit is reached.
type weightController struct {
	lastMinuteAccumulatedWeight int
	timestampOfZeroOutWeightMS  int64
	mutex                       sync.Mutex
}

var wcInstance *weightController
var lock = &sync.Mutex{}

// getWeightControllerSingleton -- constructor of weight controller. Designed as singleton.
// TODO: Refactor accoding to https://medium.com/golang-issue/how-singleton-pattern-works-with-golang-2fdd61cd5a7f
func getWeightControllerSingleton() *weightController {
	lock.Lock()
	defer lock.Unlock()

	if wcInstance == nil {
		wcInstance = &weightController{
			0,
			time.Now().Unix() * 1000,
			sync.Mutex{},
		}
	}
	return wcInstance
}

func (wcInstance *weightController) getSleepTime(requestWeight int) int64 {

	(*wcInstance).mutex.Lock()
	defer (*wcInstance).mutex.Unlock()

	currentTimestampMS := time.Now().Unix() * 1000
	elapsedTimeMS := currentTimestampMS - (*wcInstance).timestampOfZeroOutWeightMS
	recommendedSleepTime := int64(0)

	if (*wcInstance).lastMinuteAccumulatedWeight < weightLimitPerMinute && elapsedTimeMS <= sessionDurationMS {
		(*wcInstance).lastMinuteAccumulatedWeight += requestWeight
		//fmt.Printf("Accumulated Weight for current min [%s]: %d\n", time.Now().Format("15:04:05"), (*wcInstance).lastMinuteAccumulatedWeight)
	} else if (*wcInstance).lastMinuteAccumulatedWeight >= weightLimitPerMinute && elapsedTimeMS <= sessionDurationMS {
		recommendedSleepTime = sessionDurationMS - elapsedTimeMS
		//fmt.Printf("Accumulated Weight for current min [%s] is FULL: %d, recommended sleep time: %dsec\n", time.Now().Format("15:04:05"), (*wcInstance).lastMinuteAccumulatedWeight, recommendedSleepTime/1000)
	} else { // If elapsed time > 1min
		(*wcInstance).lastMinuteAccumulatedWeight = requestWeight
		(*wcInstance).timestampOfZeroOutWeightMS = currentTimestampMS
		//fmt.Printf("NEW 1-MIN REQUEST SESSION STARTED.\n")
	}

	return recommendedSleepTime
}