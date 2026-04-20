package main

import (
	"darkwebscraper/utils"
	"darkwebscraper/website"
	"os"
	"strings"
	"sync"
	"time"
)

type dataForDb struct {
	source string
	url    string
	desc   string
}

// it will have to return a map of links and descriptions
func main() {
	client := utils.ConnectToDb()
	funcs := []func(query string, chanAddDataToDb chan utils.DataForDb) bool{
		// website.Breachforums,
		// website.Gunra,
		// website.IncRansom,
		// website.Dread,
		// website.Lockbit,
		// website.LeakBase,
		// website.Darknet,
		// website.Everest,
		// website.Ransomexx,
		// website.Kairos,
		// website.Kyber, this won't work as this has captcha, if this captcha can be solved, the website can be scraped
		website.Lamashtu,
	}
	var wg sync.WaitGroup
	chanAddDataToDb := make(chan utils.DataForDb, 100)
	contents, err := os.ReadFile("names.txt")
	if err != nil {
		panic(err)
	} else {
		eachContent := strings.SplitSeq(string(contents), "\n")
		go utils.AddDataToDb(client, chanAddDataToDb)
		for i := range eachContent {
			// fmt.Println(i)
			i = strings.TrimSpace(i)
			for _, f := range funcs {
				wg.Go(func() { f(i, chanAddDataToDb) })
				// go func(fn func(query string) bool) {
				// 	defer wg.Done()
				// 	fn(i)
				// }(f)
			}
			wg.Wait()
			time.Sleep(5 * time.Second)
		}
		close(chanAddDataToDb)
	}
}
