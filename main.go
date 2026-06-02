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
	funcs := []func(chanList chan string, chanAddDataToDb chan utils.DataForDb){
		// website.Kyber, // this won't work as this has captcha, if this captcha can be solved, the website can be scraped
		// website.KillSec, // this won't work as this has captcha, if this captcha can be solved, the website can be scraped
		// website.Everest, // needs to be updated
		// website.Ransomexx, // needs to be updated
		// website.Darknet, // needs to be updated
		// website.Akira,
		// website.Ailock,

		// website.IncRansom,
		// website.Kairos,
		// website.Lamashtu,
		// website.Linkcpub,
		// website.Lynx,
		// website.MoneyMessage,
		// website.Sinobi,
		// website.Termite,
		// website.Warlock,
		// website.Morpheus,
		// website.Dread,
		// website.Lockbit,
		// website.Abyss,
		// website.DataExposureLogs,
		// website.Beast,
		// website.Atomsilo, // not working
		// website.Benzona,
		// website.Blackwater,
		// website.Braincipher,
		// website.Dragonforce,
		// website.Bashe,
		// website.Metaencryptor,
		// website.Mydata,
		// website.Icarus,
		// website.Ransomhouse,
		// website.Rhysida,
		// website.Sarcoma,
		// website.Triplex,
		website.Secpo,
		// website.PlayNews, // not working
		// website.Radar,
		// website.Fulcrumsec,
		// website.Genesis,
		// website.Ms13089,
		// website.Nova,
		// website.Payload,
		// website.Bavacai,
		// website.Dls,
		// website.Blackwater,
		// website.Cmd,
		// website.Chaos,
		// website.Coinbasecartel,
		// website.Cry0,
		// website.Daixin,
		// website.Embargo, // not working
		// website.Gunra,
		// website.Interlock,
		// website.Kazu, // not working
		// website.Krybit,
		// website.Merx,
		// website.Kazyon, // not real
		// website.Netrunner,
		// website.Nightspire, // not working
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

			wg.Go(func(ch chan string, f func(chan string, chan utils.DataForDb)) func() {
				return func() {
					f(ch, chanAddDataToDb)
				}
			}(ch, f))
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
		time.Sleep(10 * time.Second) //somehow fixes data not being uploaded
		close(chanAddDataToDb)
	}
}
