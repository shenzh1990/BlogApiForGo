package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 填写自己的Access Key和Secret Key
const (
	HuobiAccessKey = "your-huobi-access-key-here"
	HuobiSecretKey = "your-huobi-secret-key-here"

	BinanceAccessKey = "your-binance-access-key-here"
	BinanceSecretKey = "your-binance-secret-key-here"

	Symbol = "btcusdt" // 交易对
)

// 生成火币网签名字符串
func signHuobi(method, path string, params url.Values) string {
	params.Set("AccessKeyId", HuobiAccessKey)
	params.Set("SignatureMethod", "HmacSHA256")
	params.Set("SignatureVersion", "2")
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05"))
	payload := fmt.Sprintf("%s\napi.huobi.pro\n%s\n%s", method, path, params.Encode())
	mac := hmac.New(sha256.New, []byte(HuobiSecretKey))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// 发送火币网GET请求并返回响应内容
func getHuobi(path string, params url.Values) (string, error) {
	signature := signHuobi("GET", path, params)
	params.Set("Signature", signature)
	url := fmt.Sprintf("https://api.huobi.pro%s?%s", path, params.Encode())
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// 发送火币网POST请求并返回响应内容
func postHuobi(path string, params url.Values) (string, error) {
	signature := signHuobi("POST", path, params)
	params.Set("Signature", signature)
	url := fmt.Sprintf("https://api.huobi.pro%s?%s", path, params.Encode())
	resp, err := http.Post(url, "application/json", "{}")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil

}

// 生成币安网签名字符串
func signBinance(method, path string, params url.Values) string {
	query := params.Encode()
	mac := hmac.New(sha256.New, []byte(BinanceSecretKey))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}

// 发送币安网GET请求并返回响应内容
func getBinance(path string, params url.Values) (string, error) {
	signature := signBinance("GET", path, params)
	query := params.Encode() + "&signature=" + signature
	url := fmt.Sprintf("https://api.binance.com%s?%s", path.query)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("X-MBX-APIKEY", BinanceAccessKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// 发送币安网POST请求并返回响应内容
func postBinance(path string, params url.Values) (string, error) {
	signature := signBinance("POST", path, params)
	query := params.Encode() + "&signature=" + signature
	url := fmt.Sprintf("https://api.binance.com%s?%s", path.query)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("X-MBX-APIKEY", BinanceAccessKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// 获取火币网和币安网的BTC/USDT最新成交价
func getPrice() (float64, float64, error) {
	// 火币网接口
	pathHuobi := "/market/trade"
	paramsHuobi := url.Values{}
	paramsHuobi.Set("symbol", Symbol)

	// 币安网接口
	pathBinance := "/api/v3/ticker/price"
	paramsBinance := url.Values{}
	paramsBinance.Set("symbol", strings.ToUpper(Symbol))

	// 并发发送请求并获取响应内容
	ch1 := make(chan string) // 用于接收火币网的响应内容
	ch2 := make(chan string) // 用于接收币安网的响应内容

	go func() {
		result, err := getHuobi(pathHuobi, paramsHuobi)
		if err != nil {
			ch1 <- ""
		} else {
			ch1 <- result
		}
	}()

	go func() {
		result, err := getBinance(pathBinance, paramsBinance)
		if err != nil {
			ch2 <- ""
		} else {
			ch2 <- result
		}
	}()

	result1, result2 := "", ""

	select {
	case result1 = <-ch1:
	case <-time.After(5 * time.Second): // 设置超时时间为5秒，如果超过则返回错误
		return 0, 0, fmt.Errorf("timeout for huobi")
	}

	select {
	case result2 = <-ch2:
	case <-time.After(5 * time.Second): // 设置超时时间为5秒，如果超过则返回错误
		return 0, 0, fmt.Errorf("timeout for binance")
	}

	close(ch1) // 关闭通道
	close(ch2) // 关闭通道

	// 解析JSON数据并获取价格

	type HuobiResponse struct {
		Status string `json:"status"`
		Tick   struct {
			Data []struct {
				Price float64 `json:"price"`
			} `json:"data"`
		} `json:"tick"`
	}

	type BinanceResponse struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}

	var huobi HuobiResponse
	var binance BinanceResponse

	err1 := json.Unmarshal([]byte(result1), &huobi)   // 将火币网的响应内容转换为结构体对象
	err2 := json.Unmarshal([]byte(result2), &binance) // 将币安网的响应内容转换为结构体对象

	if err1 != nil || err2 != nil { // 如果有任何一个解析出错，则返回错误信息
		return 0, 0, fmt.Errorf("failed to parse json data")
	}

	if huobi.Status != "ok" { // 如果火币网的状态不是ok，则返回错误信息
		return 0, 0, fmt.Errorf("huobi status is not ok")
	}

	priceHuobi := huobi.Tick.Data[0].Price // 获取火币网的最新成交价

	priceBinance, err3 := strconv.ParseFloat(binance.Price, 64) // 将币安网的价格字符串转换为浮点数

	if err3 != nil { // 如果转换出错，则返回错误信息
		return 0, 0, fmt.Errorf("failed to parse binance price")
	}

	return priceHuobi, priceBinance, nil

}

// 在火币网和币安网之间进行搬砖操作
func arbitrage(priceHuobi, priceBinance float64) error {
	// 设置一些参数
	amount := 0.01   // 交易数量，假设为0.01个BTC
	fee := 0.002     // 交易手续费，假设为0.2%
	threshold := 100 // 价格差异阈值，假设为100美元

	// 计算价格差异
	diff := priceHuobi - priceBinance

	if diff > threshold { // 如果火币网的价格高于币安网的价格超过阈值，则在币安网买入，在火币网卖出
		fmt.Println("Buy from Binance and sell to Huobi")
		// 币安网买入接口
		pathBinance := "/api/v3/order"
		paramsBinance := url.Values{}
		paramsBinance.Set("symbol", strings.ToUpper(Symbol))
		paramsBinance.Set("side", "BUY")
		paramsBinance.Set("type", "MARKET")
		paramsBinance.Set("quantity", fmt.Sprintf("%.4f", amount))
		result1, err1 := postBinance(pathBinance, paramsBinance)
		if err1 != nil {
			return err1
		}
		fmt.Println(result1) // 打印结果

		// 火币网卖出接口
		pathHuobi := "/v1/order/orders/place"
		paramsHuobi := url.Values{}
		paramsHuobi.Set("account-id", "your-account-id-here") // 填写自己的账户ID
		paramsHuobi.Set("amount", fmt.Sprintf("%.4f", amount))
		paramsHuobi.Set("symbol", Symbol)
		paramsHuobi.Set("type", "sell-market")
		result2, err2 := postHuobi(pathHuobi, paramsHuobi)
		if err2 != nil {
			return err2
		}
		fmt.Println(result2) // 打印结果

	} else if diff <- threshold { // 如果火币网的价格低于币安网的价格超过阈值，则在火币网买入，在币安网卖出
		fmt.Println("Buy from Huobi and sell to Binance")
		// 火币网买入接口
		pathHuobi := "/v1/order/orders/place"
		paramsHuobi := url.Values{}
		paramsHuobi.Set("account-id", "your-account-id-here") // 填写自己的账户ID
		paramsHuobi.Set("amount", fmt.Sprintf("%.4f", amount))
		paramsHuobi.Set("symbol", Symbol)
		paramsHuobi.Set("type", "buy-market")
		result1, err1 := postHuobi(pathHuobi, paramsHuobi)
		if err1 != nil {
			return err1
		}
		fmt.Println(result1) // 打印结果

		// 币安网卖出接口
		pathBinance := "/api/v3/order"
		paramsBinance := url.Values{}
		paramsBinance.Set("symbol", strings.ToUpper(Symbol))
		paramsBinance.Set("side", "SELL")
		paramsBinance.Set("type", "MARKET")
		paramsBinance.Set("quantity", fmt.Sprintf("%.4f", amount))
		result2, err2 := postBinance(pathBinance, paramsBinance)
		if err2 != nil {
			return err2
		}
		fmt.Println(result2) // 打印结果

	} else { // 如果价格差异没有超过阈值，则不进行搬砖操作
		fmt.Println("No arbitrage opportunity")
	}
	return nil
}

func main() {
	for { // 无限循环，每隔一定时间检查一次价格并尝试搬砖
		priceHuobi, priceBinance, err := getPrice() // 获取火币网和币安网的最新成交价
		if err != nil {
			fmt.Println(err) // 如果获取失败，则打印错误信息并跳过本次循环
			continue
		}
		fmt.Printf("Price on Huobi: %.2f\n", priceHuobi)     // 打印火币网的价格
		fmt.Printf("Price on Binance: %.2f\n", priceBinance) // 打印币安网的价格

		err = arbitrage(priceHuobi, priceBinance) // 在火币网和币安网之间进行搬砖操作
		if err != nil {
			fmt.Println(err) // 如果搬砖失败，则打印错误信息并跳过本次循环
			continue
		}

		time.Sleep(10 * time.Second) // 暂停10秒，然后进入下一次循环

	}
}
