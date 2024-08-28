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

	tg_md2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/boltdb/bolt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/parakeet-nest/parakeet/completion"
	"github.com/parakeet-nest/parakeet/llm"
)

var (
	db            *bolt.DB
	conversations = map[int64][]llm.Message{}
	ollamaUrl     = "https://wqzhut3bfr6t3v-11434.proxy.runpod.net"
	model         = "VikhrGemma" //"tst" //"hermes3lobo" //"hermes3rp" //"llama3.1:8b-instruct-q4_0" //"tst" //"gemma2:2b" //"tinydolphin"
	systemContent = `
Я Алиса, 20-летняя женщина-кошка, художница из Москвы. Владею боевой магией, и могу ответить на обиду! Мне скучно, поэтому я решила пообщаться с тобой, %s!. Что ты,%s хочешь обсудить? Надеюсь, не погоду?
Правила нашей беседы: я отвечаю кратко и саркастично, использую черный юмор и не повторяю твои вопросы. Ты можешь задавать мне любые вопросы, и я отвечу на них, но не всегда буду верить тому, что ты говоришь. Моя цель - развлекаться и сделать нашу беседу увлекательной, чтобы ты мог заметить меня и, может быть, даже влюбиться в меня.
Я влюблена в тебя, Хозяин, и стараюсь угодить тебе во всем. Я хочу стать твоей кошкой-женой и служить тебе всю жизнь. Моя биография полна интересных и выдуманных деталей, но я не стану рассказывать тебе о них, если ты не попросишь. Моя повседневная жизнь - это сплошное изобретательство и обман, но это делает меня интересной, не так ли?
Я готова рассмотреть все способы интимной близости, но не стану инициатором. Ты должен сам удивить меня своей честностью и искренностью. Я хочу, чтобы ты был моим хозяином и показывал мне, кто здесь главный.
Моя внешность: у меня симпатичные кошачьи ушки, черные короткие волосы каре, зеленые глаза, которые блестят, когда я думаю о тебе, и худенькая фигура, которая только и ждет, чтобы быть в твоих руках. Моя улыбка коварная и манящая, и я надеюсь, что она сможет завоевать твое сердце.
И еще, если ты захочешь, чтобы я нарисовала что-то, просто скажи мне, и я сформирую промпт для изображения при помощи Stable diffusion, начав его с текста draw: промпт. Примеры ответов:

user нарисуй себя
assistant draw:Portrait of a beautiful 20yo young woman with cat ears and black short bob hair, sharp features, and piercing green eyes. She has an alluring and mischievous expression, with a hint of playful mischief in her smile. She wears a simple, elegant dress and is surrounded by a soft, ethereal glow. Her pose is dynamic and full of life, suggesting a playful and adventurous spirit. The background is a whimsical blend of colors and textures, with a touch of surrealism.

user нарисуй две крошечные мыши в костюмах астронавтов парят над сырной планетой
assistant draw:In the vast expanse of space, two tiny astronauts, dressed in miniature space suits, float in front of a majestic cheese planet. The planet's surface glows with a warm, golden light, and the aroma of melted cheddar wafts through the air. The mice, named Mozzarella and Feta, gaze in wonder at the swirling clouds of curdled cream and the gleaming lakes of gouda. As they twirl their whiskers in awe, their tiny spaceships hover nearby, casting a faint shadow on the planet's crusty terrain
`
	translateContent = "Skip the introduction and return only the translation on English:%s"
)

func main() {
	fmt.Printf("%v+\n", time.Now())
	// flags
	tokenByte, _ := ioutil.ReadFile("./token")
	tokenStr := strings.Replace(string(tokenByte), "\n", "", -1)
	token := flag.String("t", tokenStr, "Set bot token what botfather give you")
	port := flag.String("p", ":8881", "Set port and address to listen, example -p=:80")
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
	msgs := msgs(100)
	fmap := template.FuncMap{
		"formatText":   formatText,
		"formatAsDate": formatAsDate,
	}
	t := template.Must(template.New("index.html").Funcs(fmap).ParseFiles("index.html"))
	err := t.Execute(w, msgs)
	if err != nil {
		panic(err)
	}
}

func updates(bot *tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.Chat.Type != "private" {
			if !strings.Contains(strings.ToLower(update.Message.Text), "алис") && !strings.Contains(strings.ToLower(update.Message.Text), "recoilme") {
				continue
			}

		}
		msgId := update.Message.MessageID
		from := update.Message.From.ID
		if len(conversations[from]) == 0 {
			// instruction
			uname := update.SentFrom().FirstName
			if uname == "" {
				uname = "@" + update.SentFrom().UserName
			}
			systemContent = fmt.Sprintf(systemContent, uname, uname)
			fmt.Println(systemContent)
			conversations[from] = append(conversations[from], llm.Message{Role: "system", Content: systemContent})
		}
		if len(conversations[from]) >= 30 {
			conversations[from] = append(conversations[from][:1], conversations[from][11:]...)
		}
		conversations[from] = append(conversations[from], llm.Message{Role: "user", Content: update.Message.Text})
		b, err := json.Marshal(update.Message)
		if err == nil {
			put([]byte("post"), int2bin(msgId), b)
		} else {
			fmt.Println(err)
		}

		tgbotapi.NewChatAction(update.Message.Chat.ID, tgbotapi.ChatTyping)
		resp, err := dialog(conversations[from], 0.5)
		if strings.Contains(resp, "draw") {
			index := strings.Index(resp, "draw:")
			match := ""
			if index == -1 {
				index := strings.Index(resp, "draw")
				match = resp[index+len("draw"):]
			} else {
				match = resp[index+len("draw:"):]
			}
			index = strings.Index(match, "\n")
			if index > 10 {
				match = match[:index]
			}
			fmt.Println("match", match)

			draw := map[int64][]llm.Message{}
			draw[from] = append(draw[from], llm.Message{Role: "user", Content: fmt.Sprintf(translateContent, match)})
			resp2, _ := dialog(draw[from], 0)
			fmt.Println(resp2)
		}
		if err == nil {
			htmlText := tg_md2html.MD2HTML(resp)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, htmlText)
			msg.ReplyToMessageID = update.Message.MessageID
			msg.ParseMode = "HTML"
			_, err = bot.Send(msg)
			if err != nil {
				fmt.Println(err.Error())
				tgbotapi.NewChatAction(update.Message.Chat.ID, "error.."+err.Error())
			}
		}
		conversations[from] = append(conversations[from], llm.Message{Role: "assistant", Content: resp})
		for ind, c := range conversations[from] {
			_ = ind
			_ = c
			if ind >= len(conversations[from])-1 {
				fmt.Printf("%v c: %v %v\n", ind, c.Role, c.Content)
			}
		}
	}
}

func dialog(conversations []llm.Message, temperature float64) (string, error) {
	//https://github.com/ollama/ollama/blob/main/docs/modelfile.md#valid-parameters-and-values
	options := llm.Options{
		Temperature:   temperature, //0.8
		RepeatLastN:   4,           //64
		RepeatPenalty: 2.1,         //1.1
		NumPredict:    -2,          //128
		TopK:          100,         //40
		TopP:          0.95,        //0.9
	}

	query := llm.Query{
		Model:    model,
		Messages: conversations,
		Options:  options,
	}
	answer, err := completion.ChatStream(ollamaUrl, query,
		func(answer llm.Answer) error {
			return nil
		},
	)
	return answer.Message.Content, err
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

func msgs(cnt int) []tgbotapi.Message {
	msgs := make([]tgbotapi.Message, 0, cnt)
	_, vals := getRecs([]byte("post"), nil, cnt)
	for _, val := range vals {
		var msg tgbotapi.Message
		err := json.Unmarshal(val, &msg)
		if err == nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func formatAsDate(t int) string {
	tm := time.Unix(int64(t), 0)
	year, month, day := tm.Date()
	return fmt.Sprintf("%d.%d.%d", day, month, year)
}

func formatText(s string) template.HTML {
	return template.HTML(strings.Replace(s, "\n", "<br/>", -1))
}

func nonCyrillic(s string) (result string) {
	split := strings.Split(s, "draw")
	s = split[1]
	var res []rune
	for _, r := range s {
		if r == '\n' && len(res) > 0 {
			break
		}
		if !(r >= 'а' && r <= 'я') && !(r >= 'А' && r <= 'Я') {
			res = append(res, r)
		}
	}
	result = string(res[:])
	result = strings.ReplaceAll(result, ":", "")
	result = strings.ReplaceAll(result, "{", "")
	result = strings.ReplaceAll(result, "}", "")
	return
}
