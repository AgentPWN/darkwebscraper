package website

import (
	"darkwebscraper/utils"
	"fmt"
)

func LeakBase(query string, chanDataForDb chan utils.DataForDb) bool {
	fmt.Println(query)
	return true
}
