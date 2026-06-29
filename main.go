package main

import (
	"darkwebscraper/utils"
	"darkwebscraper/website"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	sourceBatchSize = 10
	torListenAddr   = "127.0.0.1:9050"
)

type dataForDb struct {
	source string
	url    string
	desc   string
}

type sourceJob struct {
	name string
	run  func(chan string, chan utils.DataForDb)
}

func torPortInUse() bool {
	conn, err := net.DialTimeout("tcp", torListenAddr, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitForTor(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if torPortInUse() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("tor did not become ready on %s within %s", torListenAddr, timeout)
}

func startTor() (*exec.Cmd, error) {
	if torPortInUse() {
		return nil, fmt.Errorf("%s is already in use; stop the manually started tor process before running this script", torListenAddr)
	}

	cmd := exec.Command("tor")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	if err := waitForTor(60 * time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, err
	}

	return cmd, nil
}

func stopTor(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func runScraperBatch(batch []sourceJob, names []string, chanAddDataToDb chan utils.DataForDb) error {
	torCmd, err := startTor()
	if err != nil {
		return err
	}
	defer stopTor(torCmd)

	var wg sync.WaitGroup
	chanList := make([]chan string, len(batch))

	for i, job := range batch {
		ch := make(chan string, 50)
		chanList[i] = ch
		done := make(chan struct{})

		wg.Go(func(ch chan string, done chan struct{}, job sourceJob) func() {
			return func() {
				defer close(done)
				defer func() {
					if recovered := recover(); recovered != nil {
						log.Printf("[%s] request failed: %v", job.name, recovered)
					}
				}()
				job.run(ch, chanAddDataToDb)
			}
		}(ch, done, job))

		go func(ch chan string, done chan struct{}) {
			defer close(ch)
			for _, name := range names {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}

				select {
				case <-done:
					return
				case ch <- name:
				}
			}
		}(ch, done)
	}

	wg.Wait()
	return nil
}

// it will have to return a map of links and descriptions
func main() {
	client := utils.ConnectToDb()
	funcs := []sourceJob{
		// website.Kyber, // this won't work as this has captcha, if this captcha can be solved, the website can be scraped
		// website.KillSec, // this won't work as this has captcha, if this captcha can be solved, the website can be scraped
		// website.Everest, // needs to be updated
		// website.Ransomexx, // needs to be updated
		// website.Darknet, // needs to be updated
		// website.Akira,
		// website.Ailock,

		// {name: "IncRansom", run: website.IncRansom},
		// {name: "Kairos", run: website.Kairos},
		// {name: "Lamashtu", run: website.Lamashtu},
		// {name: "Linkcpub", run: website.Linkcpub},
		// {name: "Lynx", run: website.Lynx},
		// {name: "MoneyMessage", run: website.MoneyMessage},
		// {name: "Sinobi", run: website.Sinobi},
		// {name: "Termite", run: website.Termite},
		// {name: "Warlock", run: website.Warlock},
		// {name: "Morpheus", run: website.Morpheus},
		// {name: "Dread", run: website.Dread},
		// {name: "Lockbit", run: website.Lockbit},
		// {name: "Abyss", run: website.Abyss},
		// {name: "DataExposureLogs", run: website.DataExposureLogs},
		// {name: "Beast", run: website.Beast},
		// {name: "Atomsilo", run: website.Atomsilo}, // not working
		// {name: "Benzona", run: website.Benzona},
		// {name: "Blackwater", run: website.Blackwater},
		// {name: "Braincipher", run: website.Braincipher},
		// {name: "Dragonforce", run: website.Dragonforce},
		// {name: "Bashe", run: website.Bashe},
		// {name: "Metaencryptor", run: website.Metaencryptor},
		// {name: "Mydata", run: website.Mydata},
		// {name: "Icarus", run: website.Icarus},
		// {name: "Ransomhouse", run: website.Ransomhouse},
		// {name: "Rhysida", run: website.Rhysida},
		// {name: "Sarcoma", run: website.Sarcoma},
		// {name: "Triplex", run: website.Triplex},
		// {name: "Secpo", run: website.Secpo},
		// {name: "PlayNews", run: website.PlayNews}, // not working
		// {name: "Radar", run: website.Radar},
		// {name: "Fulcrumsec", run: website.Fulcrumsec},
		// {name: "Genesis", run: website.Genesis},
		// {name: "Ms13089", run: website.Ms13089},
		// {name: "Nova", run: website.Nova},
		// {name: "Payload", run: website.Payload},
		// {name: "Bavacai", run: website.Bavacai},
		// {name: "Dls", run: website.Dls},
		// {name: "Blackwater", run: website.Blackwater},
		// {name: "Cmd", run: website.Cmd},
		// {name: "Chaos", run: website.Chaos},
		// {name: "Coinbasecartel", run: website.Coinbasecartel},
		// {name: "Cry0", run: website.Cry0},
		// {name: "Daixin", run: website.Daixin},
		// {name: "Embargo", run: website.Embargo}, // not working
		// {name: "Gunra", run: website.Gunra},
		// {name: "Interlock", run: website.Interlock},
		// {name: "Kazu", run: website.Kazu}, // not working
		// {name: "Krybit", run: website.Krybit},
		// {name: "Merx", run: website.Merx},
		// {name: "Kazyon", run: website.Kazyon}, // not real
		// {name: "Netrunner", run: website.Netrunner},
		// {name: "Nightspire", run: website.Nightspire}, // not working
		// {name: "Noirth", run: website.Noirth},
		{name: "PwnForums", run: website.PwnForums},
		// {name: "PayoutsKing", run: website.PayoutsKing},
		// {name: "Bravox", run: website.Bravox},
		// {name: "Bashe", run: website.Bashe},
		// {name: "Insomnia", run: website.Insomnia},
		// {name: "Anubis", run: website.Anubis},
		// {name: "Crypto24", run: website.Crypto24},
		// {name: "Direwolf", run: website.Direwolf},
	}
	chanAddDataToDb := make(chan utils.DataForDb, 100)
	contents, err := os.ReadFile("names.txt")
	if err != nil {
		panic(err)
	} else {
		names := strings.Split(strings.TrimSpace(string(contents)), "\n")

		dbDone := make(chan struct{})
		go func() {
			defer close(dbDone)
			utils.AddDataToDb(client, chanAddDataToDb)
		}()

		for start := 0; start < len(funcs); start += sourceBatchSize {
			end := start + sourceBatchSize
			if end > len(funcs) {
				end = len(funcs)
			}

			if err := runScraperBatch(funcs[start:end], names, chanAddDataToDb); err != nil {
				panic(err)
			}
		}

		close(chanAddDataToDb)
		<-dbDone
	}
}
