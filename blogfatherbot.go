package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/boltdb/bolt"

	"gopkg.in/telegram-bot-api.v4"
)

var (
	db *bolt.DB
)

func main() {
	fmt.Printf("%v+\n", time.Now())
	// flags
	tokenByte, _ := ioutil.ReadFile("./token")
	tokenStr := strings.Replace(string(tokenByte), "\n", "", -1)
	token := flag.String("t", tokenStr, "Set bot token what botfather give you")
	port := flag.String("p", ":8080", "Set port and address to listen, example -p=:80")
	debugBot := flag.Bool("d", false, "Set debug bot with flag -d=true")
	flag.Parse()
	if *token == "" {
		log.Fatal("Set bot token with flag -t=Your:Token")
	}

	// bot
	bot, err := tgbotapi.NewBotAPI(*token)
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = *debugBot
	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)
	// updates handler
	go updates(bot)

	// database
	db = initDb("db")
	defer db.Close()

	// server
	http.HandleFunc("/", handler)
	httpErr := http.ListenAndServe(*port, nil)
	if httpErr != nil {
		log.Fatal(httpErr)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
	t, _ := template.ParseFiles("index.html") //setp 1
	t.Execute(w, "Hello World!")              //step 2
}

func updates(bot *tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		fmt.Println(err)
	}
	for update := range updates {
		if update.Message == nil {
			continue
		}
		msgId := update.Message.MessageID
		b, err := json.Marshal(update.Message)
		if err == nil {
			put([]byte("post"), int2bin(msgId), b)
		} else {
			fmt.Println(err)
		}
	}
}

func put(bucket []byte, key []byte, val []byte) {
	err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			fmt.Println(err)
			return err
		}

		err = b.Put(key, val)
		if err != nil {
			fmt.Println(err)
			return err
		}
		return nil
	})

	if err != nil {
		fmt.Println(err)
	}
}

func get(bucket []byte, key []byte) []byte {
	var val []byte
	db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		val = b.Get(key)
		return nil
	})
	return val
}

func initDb(dbName string) *bolt.DB {
	db, err := bolt.Open(dbName, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal("db:", err)
	}
	return db
}

func int2bin(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func getRecs(bucket []byte, from []byte, cnt int) ([][]byte, [][]byte) {

	keys := make([][]byte, 0, cnt)
	vals := make([][]byte, 0, cnt)
	var fromKey = from
	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		var skipFirst = true
		if fromKey == nil {
			skipFirst = false
			fromKey, _ = c.Last()
		} else {
			cnt++
		}
		total := 0
		for k, v := c.Seek(fromKey); k != nil; k, v = c.Prev() {
			if total == 0 && skipFirst {
				total++
				continue
			}
			if total >= cnt {
				break
			}
			vals = append(vals, v)
			keys = append(keys, k)
			total++
		}
		return nil
	})
	return keys, vals
}
