package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"gopkg.in/telegram-bot-api.v4"
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

	go updates(bot)

	// server
	http.HandleFunc("/", handler)
	httpErr := http.ListenAndServe(*port, nil)
	if httpErr != nil {
		log.Fatal(httpErr)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
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
	}
}
