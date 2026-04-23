package main

import (
	"darkwebscraper/utils"
	"darkwebscraper/website"
	"os"
	"strings"
	"sync"
)

type dataForDb struct {
	source string
	url    string
	desc   string
}

// it will have to return a map of links and descriptions
func main() {
	client := utils.ConnectToDb()
	funcs := []func(chanList chan string, chanAddDataToDb chan utils.DataForDb){
		// website.Kyber, // this won't work as this has captcha, if this captcha can be solved, the website can be scraped
		// website.KillSec, // this won't work as this has captcha, if this captcha can be solved, the website can be scraped
		// website.Gunra,
		// website.IncRansom,
		// website.Dread,
		// website.Lockbit,
		// website.Darknet,
		// website.Everest,
		// website.Ransomexx,
		// website.Kairos,
		// website.Lamashtu,
		// website.Linkcpub,
		// website.Lynx,
		// website.MoneyMessage,
		// website.Morpheus,
		website.Sinobi,
		website.Termite,
		website.Warlock,
	}
	var wg sync.WaitGroup
	chanList := make([]chan string, len(funcs))
	chanAddDataToDb := make(chan utils.DataForDb, 100)
	contents, err := os.ReadFile("names.txt")
	if err != nil {
		panic(err)
	} else {
		eachContent := strings.SplitSeq(string(contents), "\n")
		go utils.AddDataToDb(client, chanAddDataToDb)
		// for i := range eachContent {
		// 	// fmt.Println(i)
		// 	i = strings.TrimSpace(i)
		// 	for _, f := range funcs {
		// 		wg.Go(func() { f(i, chanAddDataToDb) })
		// 		// go func(fn func(query string) bool) {
		// 		// 	defer wg.Done()
		// 		// 	fn(i)
		// 		// }(f)
		// 	}
		// 	wg.Wait()
		// 	time.Sleep(5 * time.Second)
		// }
		for i, f := range funcs {
			ch := make(chan string, 50)
			chanList[i] = ch
			wg.Go(func() { f(chanList[i], chanAddDataToDb) })
			// fmt.Println(i, s)
		}
		for i := range eachContent {
			for j, _ := range funcs {
				chanList[j] <- i
				// time.Sleep(100 * time.Millisecond)
			}
		}
		for i, _ := range funcs {
			close(chanList[i])
		}
		wg.Wait()
		close(chanAddDataToDb)
	}
}
