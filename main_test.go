package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestDb(t *testing.T) {
	bucket := "bucket"
	key := "key"
	val := "val"
	db = initDb("tst")
	put([]byte(bucket), []byte(key), []byte(val))
	valGet := get([]byte(bucket), []byte(key))
	fmt.Println(string(valGet))
	if val != string(valGet) {
		t.Error("Expected "+val+", got ", string(valGet))
	}
	defer db.Close()
}

func TestBigEndian(t *testing.T) {
	db = initDb("tst")
	defer db.Close()
	bucket := "bucketint"
	for i := 1; i <= 20; i++ {
		put([]byte(bucket), int2bin(i), int2bin(i))
	}
	keys, _ := getRecs([]byte(bucket), nil, 10)
	var last []byte
	for _, key := range keys {
		fmt.Printf("key:%v\n", binary.BigEndian.Uint64(key))
		last = key
	}
	keysNext, _ := getRecs([]byte(bucket), last, 10)
	for _, key := range keysNext {
		fmt.Printf("key:%v\n", binary.BigEndian.Uint64(key))
		last = key
	}
	if 1 != binary.BigEndian.Uint64(last) {
		t.Error("Expected, got ", binary.BigEndian.Uint64(last))
	}
}

func TestPrintLast(t *testing.T) {
	db = initDb("db")
	defer db.Close()
	bucket := "post"
	_, vals := getRecs([]byte(bucket), nil, 3)
	for _, val := range vals {
		var msg tgbotapi.Message
		err := json.Unmarshal(val, &msg)
		if err == nil {
			fmt.Printf("val:%+v\n", msg)
		}
	}
}

func TestNonCyrillic(t *testing.T) {
	s := `111
		нарисуй draw:{cow jy, blodd 111 }aasыыs
	 ww олень`
	r := nonCyrillic(s)
	fmt.Println(r)
}
