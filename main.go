package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-resty/resty/v2"
)

var listenFilePath = "./demo.text"
var defLanguage = "zh-CN"
var languageTag = []string{"en", "fr"} // 英语 法语
var suffix = ".text"
var youdaoCookie = "OUTFOX_SEARCH_USER_ID=-1542349537@10.108.162.135; JSESSIONID=aaapDlaae38PY-VoYGY9x; OUTFOX_SEARCH_USER_ID_NCOO=610698240.2582016; fanyi-ad-id=305002; fanyi-ad-closed=1; ___rl__test__cookies="

var defTextList = []string{} // 默认的文本内容
var newTextList = []string{} //用来匹对的内容
var reg = regexp.MustCompile(`<span[^>][\s]*id="tw-answ-target-text"[\s]*[\>](?s:(.*?))<\/span>`)

func Change(fileName string) {
	newTextList = []string{}
	log.Println("modified file：", fileName)

	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0)

	if err != nil {
		log.Println("打开文件失败：", fileName)
		return
	}

	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		newTextList = append(newTextList, strings.TrimSpace(sc.Text()))
	}

	if len(newTextList) == 0 {
		// 有时候读取会有问题
		return
	}

	// fmt.Printf("newTextList %v \n", newTextList)
	// fmt.Printf("defTextList %v \n", defTextList)
	tmp := make([]string, len(newTextList), (cap(newTextList))*2)

	if len(defTextList) == 0 {
		copy(tmp, newTextList)
		defTextList = tmp
		// 所有都要翻译
		for _, e := range languageTag {
			go FileTranslation(e, e, defTextList, true)
		}
	} else {
		// 匹对两个切片的不同元素 放到翻译
		addList := Compare(defTextList, newTextList)
		delList := Compare(newTextList, defTextList)
		for _, e := range languageTag {
			go FileTranslation(e, e, delList, false)
			go FileTranslation(e, e, addList, true)
		}

		defer func() {
			copy(tmp, newTextList)
			defTextList = tmp
		}()
	}

}

func FileTranslation(fileName string, language string, list []string, isAdd bool) {

	if len(list) == 0 {
		return
	}

	if !isAdd {
		DelKey(fileName, list)
		return
	}

	path := fileName + suffix
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		fmt.Printf("文件 %s 打开失败 \n", path)
		return
	}

	defer f.Close()

	// 直接追加
	write := bufio.NewWriter(f)
	for _, e := range list {

		sp := strings.Split(e, "=")
		if len(sp) < 2 {
			continue
		}

		s := YoudaoTranslation(language, sp[1])
		write.WriteString(sp[0] + "=" + s + "\r\n")
	}
	write.Flush()
}

func DelKey(fileName string, delList []string) {
	path := fileName + suffix
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0)

	if err != nil {
		log.Println("打开文件失败：", fileName)
		return
	}

	defer f.Close()

	tmp := map[string]int{}
	for _, e := range delList {
		tmp[strings.Split(e, "=")[0]] = 1
	}

	oldList := []string{}
	sc := bufio.NewScanner(f)

	for sc.Scan() {
		s := strings.TrimSpace(sc.Text())
		sp := strings.Split(s, "=")
		v := tmp[sp[0]]
		if v != 1 {
			oldList = append(oldList, s)
		}
	}

	err = os.Truncate(path, 0)
	if err != nil {
		log.Fatal(err)
	}
	f.Seek(0, 0)

	// fmt.Printf("删除文件 %v", oldList)
	write := bufio.NewWriter(f)
	for _, e := range oldList {
		write.WriteString(e + "\r\n")
	}
	write.Flush()

}

// google有道翻译
func GoogleYoudaoTranslation(language string, text string) string {
	client := resty.New()
	resStr := ""

	// form 表单提交
	resp, err := client.R().
		SetHeader("origin", "https://www.google.com").
		SetHeader("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.102 Safari/537.36").
		SetFormData(map[string]string{
			"async": "translate,sl:" + defLanguage + ",tl:" + language + ",st:" + text + ",id:1646793676227,qc:true,ac:true,_id:tw-async-translate,_pms:s,_fmt:pc",
		}).
		SetResult(&resStr).
		Post("https://www.google.com/async/translate?vet=12ahUKEwiL6_6d9Lr2AhWURd4KHUnDB0UQqDh6BAgCECY..i&ei=lZkpYsvMF5SL-QbJhp-oBA&rlz=1C1GCEU_zh-TWTW977TW977&yv=3")

	if err != nil {
		return ""
	}

	str := string(resp.Body())
	res := reg.FindAllStringSubmatch(str, -1)

	if res == nil {
		return ""
	}
	return res[0][1]
}

// 国内有道翻译
func YoudaoTranslation(language string, text string) string {
	t := MD5("5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.102 Safari/537.36")
	r := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	i := r + strconv.Itoa(rand.Intn(10))
	sign := MD5("fanyideskweb" + text + i + "Ygy_4c=r#e#4EX^NUGUc5")
	client := resty.New()

	var data = map[string]string{
		"i":           text,
		"from":        defLanguage,
		"to":          language,
		"smartresult": "dict",
		"client":      "fanyideskweb",
		"salt":        i,
		"sign":        sign,
		"lts":         r,
		"bv":          t,
		"doctype":     "json",
		"version":     "2.1",
		"keyfrom":     "fanyi.web",
		"action":      "FY_BY_CLICKBUTTION",
	}
	var rest = Result{}
	// form 表单提交
	_, err := client.R().
		SetHeader("origin", "https://fanyi.youdao.com").
		SetHeader("Referer", "https://fanyi.youdao.com/").
		SetHeader("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8").
		SetHeader("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.102 Safari/537.36").
		SetHeader("Cookie", youdaoCookie+r).
		// SetCookies(cookies).
		SetFormData(data).
		SetContentLength(true).
		SetResult(&rest).
		// SetBody(map[string]interface{}{"async": "translate,sl:auto,tl:en,st:你好,id:1646793676227,qc:true,ac:true,_id:tw-async-translate,_pms:s,_fmt:pc"}).
		Post("https://fanyi.youdao.com/translate_o?smartresult=dict&smartresult=rule")

	if err != nil {
		fmt.Println(err)
		return ""
	}

	if rest.ErrorCode != 0 {
		return ""
	}

	// fmt.Println("resp", rest.TranslateResult[0][0].Tgt)
	return rest.TranslateResult[0][0].Tgt
}

func Compare(defList []string, newList []string) []string {
	temMap := map[string]int{}
	result := []string{}

	for _, e := range defList {
		temMap[e] = 1
	}

	for _, e := range newList {
		tem := temMap[e]
		if tem == 1 {
			continue
		}

		result = append(result, e)
	}

	return result
}

//读取key=value类型的配置文件
func InitConfig(path string) map[string]string {
	config := make(map[string]string)

	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	r := bufio.NewReader(f)
	for {
		b, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		s := strings.TrimSpace(string(b))
		index := strings.Index(s, "=")
		if index < 0 {
			continue
		}
		key := strings.TrimSpace(s[:index])
		if len(key) == 0 {
			continue
		}
		value := strings.TrimSpace(s[index+1:])
		if len(value) == 0 {
			continue
		}
		config[key] = value
	}
	return config
}

func MD5(text string) string {
	d := []byte(text)
	m := md5.New()
	m.Write(d)
	return hex.EncodeToString(m.Sum(nil))
}

type Result struct {
	ErrorCode       int           `json:"errorCode"`
	TranslateResult [][]Translate `json:"translateResult"`
	Type            string        `json:"type"`
}

type Translate struct {
	Tgt string `json:"tgt"`
	Src string `json:"src"`
}

func main() {

	// 读取配置文件
	config := InitConfig("./config.text")
	fmt.Printf("config的配置为： %v \n", config)

	if value, ok := config["listenFilePath"]; ok {
		listenFilePath = value
	}

	if value, ok := config["defLanguage"]; ok {
		defLanguage = value
	}

	if value, ok := config["languageTag"]; ok {
		languageTag = strings.Split(value, ",")
	}

	if value, ok := config["suffix"]; ok {
		suffix = value
	}

	if value, ok := config["cookie"]; ok {
		youdaoCookie = value
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					Change(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(listenFilePath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

