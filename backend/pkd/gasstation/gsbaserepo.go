/*
  - Copyright 2022 Sven Loesekann
    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
*/
package gasstation

import (
	"fmt"
	"log"
	"os"
	"react-and-go/pkd/database"
	"react-and-go/pkd/gasstation/gsmodel"
	"react-and-go/pkd/notification"
	"react-and-go/pkd/postcode/pcmodel"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

//const earthRadius = 6371.0

type TimeFrame uint

const (
	Day   TimeFrame = iota
	Month TimeFrame = iota
)

type minMaxSquare struct {
	MinLat float64
	MinLng float64
	MaxLat float64
	MaxLng float64
}

func resetDataMaps(stateDataMapRef *map[int]pcmodel.StateData, countyDataMap *map[int]pcmodel.CountyData) {
	idCountyDataMap := *countyDataMap
	for key, myCounty := range idCountyDataMap {
		myCounty.AvgDiesel = 0
		myCounty.AvgE10 = 0
		myCounty.AvgE5 = 0
		myCounty.GasStationNum = 0
		myCounty.GsNumDiesel = 0
		myCounty.GsNumE10 = 0
		myCounty.GsNumE5 = 0
		idCountyDataMap[key] = myCounty
	}
	idStateDataMap := *stateDataMapRef
	for key, myState := range idStateDataMap {
		myState.AvgDiesel = 0
		myState.AvgE10 = 0
		myState.AvgE5 = 0
		myState.GasStationNum = 0
		myState.GsNumDiesel = 0
		myState.GsNumE10 = 0
		myState.GsNumE5 = 0
		idStateDataMap[key] = myState
	}
}

func createPostCodeMaps() (map[int]pcmodel.PostCodeLocation, map[int]pcmodel.StateData, map[int]pcmodel.CountyData) {
	var postcodeLocations []pcmodel.PostCodeLocation
	database.DB.Preload("StateData").Preload("CountyData").Find(&postcodeLocations)
	postCodePostCodeLocationMap := make(map[int]pcmodel.PostCodeLocation)
	idStateDataMap := make(map[int]pcmodel.StateData)
	idCountyDataMap := make(map[int]pcmodel.CountyData)
	for _, myPostcodeLocation := range postcodeLocations {
		postCodePostCodeLocationMap[int(myPostcodeLocation.PostCode)] = myPostcodeLocation
		idStateDataMap[int(myPostcodeLocation.StateData.ID)] = myPostcodeLocation.StateData
		idCountyDataMap[int(myPostcodeLocation.CountyData.ID)] = myPostcodeLocation.CountyData
	}
	return postCodePostCodeLocationMap, idStateDataMap, idCountyDataMap
}

func createGasStationIdGasPriceMap(gasStations *[]gsmodel.GasStation, timeframe TimeFrame) map[string][]gsmodel.GasPrice {
	gasPrices := findGasPricesByTimeframe(gasStations, timeframe)
	//log.Printf("gasPrices: %v", len(gasPrices))
	gasStationIdGasPriceMap := make(map[string][]gsmodel.GasPrice)
	for _, myGasPrice := range gasPrices {
		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local).Round(time.Hour)
		if _, ok := gasStationIdGasPriceMap[myGasPrice.GasStationID]; !ok && myGasPrice.Date.Before(today) {
			var myGasPrices []gsmodel.GasPrice
			gasStationIdGasPriceMap[myGasPrice.GasStationID] = append(myGasPrices, myGasPrice)
		} else if myGasPrice.Date.Before(today) {
			gasStationIdGasPriceMap[myGasPrice.GasStationID] = append(gasStationIdGasPriceMap[myGasPrice.GasStationID], myGasPrice)
		}
	}
	return gasStationIdGasPriceMap
}

func createGasStationIdGasPriceArrayMap(gasStations *[]gsmodel.GasStation, timeframe TimeFrame) map[string][]gsmodel.GasPrice {
	gasPrices := findGasPricesByTimeframe(gasStations, timeframe)
	//log.Printf("gasPrices: %v", len(gasPrices))
	gasStationIdGasPriceMap := make(map[string][]gsmodel.GasPrice)
	for _, myGasPrice := range gasPrices {
		if _, ok := gasStationIdGasPriceMap[myGasPrice.GasStationID]; !ok {
			gasStationIdGasPriceMap[myGasPrice.GasStationID] = make([]gsmodel.GasPrice, 1)
			gasStationIdGasPriceMap[myGasPrice.GasStationID][0] = myGasPrice
		} else {
			gasStationIdGasPriceMap[myGasPrice.GasStationID] = append(gasStationIdGasPriceMap[myGasPrice.GasStationID], myGasPrice)
		}
	}
	return gasStationIdGasPriceMap
}

func findGasPricesByTimeframe(gasStations *[]gsmodel.GasStation, timeframe TimeFrame) []gsmodel.GasPrice {
	var gasStationIds []string
	for _, myGasStation := range *gasStations {
		gasStationIds = append(gasStationIds, myGasStation.ID)
	}
	//log.Printf("gasStationIds: %v", len(gasStationIds))
	gasPrices := FindPricesByStids(&gasStationIds, 0, timeframe, false)
	return gasPrices
}

func createPostCodeGasStationsMap() map[string][]gsmodel.GasStation {
	var gasStations []gsmodel.GasStation
	database.DB.Find(&gasStations)
	postCodeGasStationsMap := make(map[string][]gsmodel.GasStation)
	for _, myGasStation := range gasStations {
		postCodeGasStationsMap[myGasStation.PostCode] = append(postCodeGasStationsMap[myGasStation.PostCode], myGasStation)
	}
	return postCodeGasStationsMap
}

func findPricesByStids(stids *[]string, resultLimit int, timeframe TimeFrame, onlyLastUpdate bool) []gsmodel.GasPrice {
	var myGasPrices []gsmodel.GasPrice
	var myTimeFrame time.Time
	if timeframe == Day {
		myTimeFrame = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -1).Round(time.Hour)
	} else {
		myTimeFrame = time.Now().Add(time.Hour * -720)
	}
	dateStr := fmt.Sprintf("%04d-%02d-%02d", myTimeFrame.Year(), myTimeFrame.Month(), myTimeFrame.Day())
	//log.Printf("Cut off date: %v", dateStr)
	chuncks := createInChunks(stids, true)
	database.DB.Transaction(func(tx *gorm.DB) error {
		for _, chunk := range chuncks {
			var values []gsmodel.GasPrice
			//log.Printf("Chunk: %v\n", chunk)
			myQuery := tx.Where("stid IN ? and date >= date(?) ", chunk, dateStr).Order("date desc")
			if resultLimit > 0 {
				myQuery.Limit(resultLimit)
			}
			myQuery.Find(&values)
			//log.Printf("%v", values)
			var lastUpdates = values
			if onlyLastUpdate {
				lastUpdates = returnLastUpdatesGasStation(&values)
			}
			//log.Printf("%v", lastUpdates)
			myGasPrices = append(myGasPrices, lastUpdates...)
		}
		return nil
	})
	return myGasPrices
}

func returnLastUpdatesGasStation(values *[]gsmodel.GasPrice) []gsmodel.GasPrice {
	var myGasPrices []gsmodel.GasPrice
	gasPriceUpdateMap := make(map[string]gsmodel.GasPrice)
	for _, value := range *values {
		if entry, ok := gasPriceUpdateMap[value.GasStationID]; ok {
			if entry.Date.Before(value.Date) {
				gasPriceUpdateMap[value.GasStationID] = value
			}
		} else {
			gasPriceUpdateMap[value.GasStationID] = value
		}
	}
	for _, value := range gasPriceUpdateMap {
		myGasPrices = append(myGasPrices, value)
	}
	return myGasPrices
}

func createNewGasStation(value GasStationImport) gsmodel.GasStation {
	var resultGs gsmodel.GasStation
	resultGs.ID = value.Uuid
	resultGs.Brand = value.Brand
	resultGs.FirstActive = value.FirstActive
	resultGs.HouseNumber = value.HouseNumber
	resultGs.Latitude = value.Latitude
	resultGs.Longitude = value.Longitude
	resultGs.OpenTs = 0
	resultGs.OtJson = value.OpeningTimesJson
	resultGs.Place = value.City
	resultGs.PostCode = value.PostCode
	resultGs.PriceChanged = time.Now()
	resultGs.PriceInImport = time.Now()
	//resultGs.PublicHolidayIdentifier = ""
	resultGs.StationInImport = time.Now()
	resultGs.StationName = value.StationName
	resultGs.Street = value.Street
	resultGs.Version = "1"
	resultGs.VersionTime = time.Now()
	return resultGs
}

func createPostCodePriceMap(gasStationIDToGasPriceMap *map[string]gsmodel.GasPrice) map[string]gsmodel.GasStation {
	var gasStationIDs []string
	for gasStationID := range *gasStationIDToGasPriceMap {
		gasStationIDs = append(gasStationIDs, gasStationID)
	}
	gasStationIDChunks := createChunks(&gasStationIDs)
	postcodeGasPriceMap := make(map[string]gsmodel.GasStation)
	for _, gasStationIDChunk := range gasStationIDChunks {
		var values []gsmodel.GasStation
		database.DB.Where("ID IN ?", gasStationIDChunk).Find(&values)
		for _, myGasStation := range values {
			postcodeGasPriceMap[myGasStation.PostCode] = myGasStation
		}
	}
	return postcodeGasPriceMap
}

func createPostcodePostcodeLocationMap(postcodeGasPriceMap *map[string]gsmodel.GasStation) map[int]pcmodel.PostCodeLocation {
	var postcodes []string
	for myPostcode := range *postcodeGasPriceMap {
		if len(strings.TrimSpace(myPostcode)) == 0 {
			continue
		}
		postcodes = append(postcodes, myPostcode)
	}
	//var postcodeLocations []pcmodel.PostCodeLocation
	postcodePostcodeLocationMap := make(map[int]pcmodel.PostCodeLocation)
	postcodeChunks := createChunks(&postcodes)
	for _, myPostcode := range postcodeChunks {
		var values []pcmodel.PostCodeLocation
		database.DB.Where("post_code IN ?", myPostcode).Preload("StateData").Preload("CountyData").Find(&values)
		//postcodeLocations = append(postcodeLocations, values...)
		for _, myValue := range values {
			postcodePostcodeLocationMap[int(myValue.PostCode)] = myValue
		}
	}
	return postcodePostcodeLocationMap
}

func createPostCodeGasStationMaps() (map[int]pcmodel.PostCodeLocation, map[int]pcmodel.StateData, map[int]pcmodel.CountyData, map[string][]gsmodel.GasStation) {
	postCodePostCodeLocationMap, idStateDataMap, idCountyDataMap := createPostCodeMaps()
	log.Printf("postCodePostCodeLocationMap: %v, idStateDataMap: %v, idCountyDataMap: %v",
		len(postCodePostCodeLocationMap), len(idStateDataMap), len(idCountyDataMap))
	postCodeGasStationsMap := createPostCodeGasStationsMap()
	log.Printf("postCodeGasStationsMap: %v", len(postCodeGasStationsMap))
	return postCodePostCodeLocationMap, idStateDataMap, idCountyDataMap, postCodeGasStationsMap
}

func updateCountyStatePrices(gasStationIDToGasPriceMap *map[string]gsmodel.GasPrice) int {
	postcodeGasPriceMap := createPostCodePriceMap(gasStationIDToGasPriceMap)
	postcodePostcodeLocationMap := createPostcodePostcodeLocationMap(&postcodeGasPriceMap)
	modifiedStatesMap := make(map[int]pcmodel.StateData)
	modifiedCountiesMap := make(map[int]pcmodel.CountyData)
	//update avg prices
	for myPostcode, myGasStation := range postcodeGasPriceMap {
		myPostCodeInt, err := strconv.Atoi(myPostcode)
		if err != nil {
			continue
		}
		myPostcodeLocation := postcodePostcodeLocationMap[myPostCodeInt]
		myGasStationIDToGasPriceMap := *gasStationIDToGasPriceMap
		myGasprice := myGasStationIDToGasPriceMap[myGasStation.ID]
		//calc CountData by subtracting the the average fraction and adding the new price fraction
		if myPostcodeLocation.CountyData.GasStationNum > 0 {
			if _, ok := modifiedCountiesMap[int(myPostcodeLocation.CountyData.ID)]; !ok {
				modifiedCountiesMap[int(myPostcodeLocation.CountyData.ID)] = myPostcodeLocation.CountyData
			}
			myCountyData := modifiedCountiesMap[int(myPostcodeLocation.CountyData.ID)]
			if myGasprice.Diesel > 10 && myCountyData.GsNumDiesel > 0 {
				myCountyData.AvgDiesel += float64(myGasprice.Diesel)/float64(myCountyData.GsNumDiesel) - myCountyData.AvgDiesel/float64(myCountyData.GsNumDiesel)
			}
			if myGasprice.E10 > 10 && myCountyData.GsNumE10 > 0 {
				myCountyData.AvgE10 += float64(myGasprice.E10)/float64(myCountyData.GsNumE10) - myCountyData.AvgE10/float64(myCountyData.GsNumE10)
			}
			if myGasprice.E5 > 10 && myCountyData.GsNumE5 > 0 {
				myCountyData.AvgE5 += float64(myGasprice.E5)/float64(myCountyData.GsNumE5) - myCountyData.AvgE5/float64(myCountyData.GsNumE5)
			}
			modifiedCountiesMap[int(myPostcodeLocation.CountyData.ID)] = myCountyData
		}
		//calc StateData by subtracting the the average fraction and adding the new price fraction
		if myPostcodeLocation.StateData.GasStationNum > 0 {
			if _, ok := modifiedStatesMap[int(myPostcodeLocation.StateData.ID)]; !ok {
				modifiedStatesMap[int(myPostcodeLocation.StateData.ID)] = myPostcodeLocation.StateData
			}
			myStateData := modifiedStatesMap[int(myPostcodeLocation.StateData.ID)]
			if myGasprice.Diesel > 10 && myStateData.GsNumDiesel > 0 {
				myStateData.AvgDiesel += float64(myGasprice.Diesel)/float64(myStateData.GsNumDiesel) - myStateData.AvgDiesel/float64(myStateData.GsNumDiesel)
			}
			if myGasprice.E10 > 10 && myStateData.GsNumE10 > 0 {
				myStateData.AvgE10 += float64(myGasprice.E10)/float64(myStateData.GsNumE10) - myStateData.AvgE10/float64(myStateData.GsNumE10)
			}
			if myGasprice.E5 > 10 && myStateData.GsNumE5 > 0 {
				myStateData.AvgE5 += float64(myGasprice.E5)/float64(myStateData.GsNumE5) - myStateData.AvgE5/float64(myStateData.GsNumE5)
			}
			modifiedStatesMap[int(myPostcodeLocation.StateData.ID)] = myStateData
		}
	}
	database.DB.Transaction(func(tx *gorm.DB) error {
		for _, myStateData := range modifiedStatesMap {
			tx.Save(myStateData)
		}
		for _, myCountyData := range modifiedCountiesMap {
			tx.Save(myCountyData)
		}
		return nil
	})
	return len(postcodePostcodeLocationMap)
}

func sendNotifications(gasStationIDToGasPriceMap *map[string]gsmodel.GasPrice) {
	var gasStationIds []string
	for key := range *gasStationIDToGasPriceMap {
		gasStationIds = append(gasStationIds, key)
	}
	gasStations := findByIds(&gasStationIds)
	notification.SendNotifications(gasStationIDToGasPriceMap, gasStations)
}

func createInChunks(ids *[]string, chunkedSelects bool) [][]string {
	chunkSize := 500
	if chunkedSelects {
		chunkSize = 249
	}
	chuncks := chunkSlice(*ids, chunkSize)
	if len(chuncks) > 1 {
		log.Printf("Number of Chunks: %v\n", len(chuncks))
	}
	return chuncks
}

func createChunks(ids *[]string) [][]string {
	cunckedSelects := strings.ToLower(strings.TrimSpace(os.Getenv("DB_CHUNKED_SELECTS")))
	return createInChunks(ids, cunckedSelects == "true")
}

func findByIds(ids *[]string) []gsmodel.GasStation {
	var result []gsmodel.GasStation
	chuncks := createChunks(ids)
	database.DB.Transaction(func(tx *gorm.DB) error {
		for _, chunk := range chuncks {
			var values []gsmodel.GasStation
			tx.Where("id in ?", chunk).Find(&values)
			result = append(result, values...)
		}
		return nil
	})
	return result
}

func calcMinMaxSquare(longitude float64, latitude float64, radius float64) minMaxSquare {
	minMax := minMaxSquare{MinLat: 1000.0, MinLng: 1000.0, MaxLat: 0.0, MaxLng: 0.0}
	//fmt.Printf("StartLat: %v, StartLng: %v Radius: %v\n", searchLocation.Latitude, searchLocation.Longitude, searchLocation.Radius)
	//max supported radius 20km and add 0.1 for floation point side effects
	northLat, northLng := gsmodel.CalcLocation(latitude, longitude, radius, 0.0)
	minMax = updateMinMaxSquare(northLat, northLng, minMax)
	//fmt.Printf("NorthLat: %v, NorthLng: %v\n", northLat, northLng)
	eastLat, eastLng := gsmodel.CalcLocation(latitude, longitude, radius, 90.0)
	minMax = updateMinMaxSquare(eastLat, eastLng, minMax)
	//fmt.Printf("EastLat: %v, EastLng: %v\n", eastLat, eastLng)
	southLat, southLng := gsmodel.CalcLocation(latitude, longitude, radius, 180.0)
	minMax = updateMinMaxSquare(southLat, southLng, minMax)
	//fmt.Printf("SouthLat: %v, SouthLng: %v\n", southLat, southLng)
	westLat, westLng := gsmodel.CalcLocation(latitude, longitude, radius, 270.0)
	minMax = updateMinMaxSquare(westLat, westLng, minMax)
	return minMax
}

func chunkSlice[T any](mySlice []T, chunkSize int) (s [][]T) {
	numberOfChunks := len(mySlice)/chunkSize + 1
	var result [][]T
	for i := 0; i < numberOfChunks; i++ {
		min := (i * len(mySlice) / numberOfChunks)
		max := ((i + 1) * len(mySlice)) / numberOfChunks
		result = append(result, mySlice[min:max])
	}
	return result
}

func updateMinMaxSquare(newLat float64, newLng float64, minMax minMaxSquare) minMaxSquare {
	if newLat > minMax.MaxLat {
		minMax.MaxLat = newLat
	}
	if newLat < minMax.MinLat {
		minMax.MinLat = newLat
	}
	if newLng > minMax.MaxLng {
		minMax.MaxLng = newLng
	}
	if newLng < minMax.MinLng {
		minMax.MinLng = newLng
	}
	return minMax
}
