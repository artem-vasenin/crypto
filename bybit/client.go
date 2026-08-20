package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"cs/model"
)

const (
	// Основной публичный API Bybit.
	baseURL = "https://api.bybit.com"

	// Для нашего скринера используем USDT perpetual.
	category = "linear"
)

// Client — HTTP-клиент Bybit.
//
// Один экземпляр Client используется всем приложением.
// Это важно: http.Client умеет переиспользовать TCP-соединения,
// поэтому мы не создаём новый HTTP-клиент на каждый запрос.
type Client struct {
	httpClient *http.Client

	// Максимальное количество повторных попыток.
	maxRetries int
}

// NewClient создаёт новый клиент Bybit.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			// Если Bybit по какой-то причине зависнет,
			// программа не должна ждать бесконечно.
			Timeout: 15 * time.Second,

			// Transport оставляем стандартным.
			// net/http сам умеет переиспользовать соединения.
		},

		maxRetries: 3,
	}
}

// apiResponse — общая оболочка ответа Bybit.
//
// Практически все V5 endpoint'ы используют retCode / retMsg / result.
type apiResponse[T any] struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  T      `json:"result"`
}

// do выполняет GET-запрос к Bybit.
//
// Это центральная функция клиента.
//
// Все endpoint'ы используют её, поэтому timeout,
// retry и обработка ошибок находятся в одном месте.
func (c *Client) do(
	ctx context.Context,
	path string,
	params url.Values,
	target any,
) error {

	requestURL := baseURL + path

	if len(params) > 0 {
		requestURL += "?" + params.Encode()
	}

	var lastErr error

	for attempt := 0; attempt < c.maxRetries; attempt++ {

		// Создаём запрос с переданным context.
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			requestURL,
			nil,
		)

		if err != nil {
			return err
		}

		// Просим Bybit вернуть JSON.
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)

		if err != nil {
			lastErr = err

			// Небольшая задержка перед повторной попыткой.
			if attempt+1 < c.maxRetries {
				time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
			}

			continue
		}

		// Очень важно закрывать Body.
		// Благодаря этому HTTP Transport сможет переиспользовать
		// соединение для следующих запросов.
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf(
				"bybit HTTP status: %s",
				resp.Status,
			)

			if attempt+1 < c.maxRetries {
				time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
			}

			continue
		}

		decoder := json.NewDecoder(resp.Body)

		if err := decoder.Decode(target); err != nil {
			lastErr = fmt.Errorf(
				"decode Bybit response: %w",
				err,
			)

			if attempt+1 < c.maxRetries {
				time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
			}

			continue
		}

		return nil
	}

	return fmt.Errorf(
		"Bybit request failed after %d attempts: %w",
		c.maxRetries,
		lastErr,
	)
}

// -----------------------------------------------------------------------------
// Instruments
// -----------------------------------------------------------------------------

type instrumentsResult struct {
	NextPageCursor string `json:"nextPageCursor"`

	List []struct {
		Symbol       string `json:"symbol"`
		BaseCoin     string `json:"baseCoin"`
		QuoteCoin    string `json:"quoteCoin"`
		ContractType string `json:"contractType"`
		Status       string `json:"status"`

		LaunchTime   string `json:"launchTime"`
		DeliveryTime string `json:"deliveryTime"`

		FundingIntervalHour int `json:"fundingIntervalHour"`
	} `json:"list"`
}

// GetInstruments получает все активные USDT perpetual контракты.
//
// Важный момент:
// Bybit сейчас имеет больше 500 linear-инструментов,
// поэтому один запрос недостаточен.
// Мы обязательно обрабатываем nextPageCursor.
func (c *Client) GetInstruments(
	ctx context.Context,
) ([]model.Instrument, error) {

	var result []model.Instrument

	var cursor string

	for {
		params := url.Values{}

		params.Set("category", category)
		params.Set("status", "Trading")
		params.Set("limit", "1000")

		if cursor != "" {
			params.Set("cursor", cursor)
		}

		var response apiResponse[instrumentsResult]

		if err := c.do(
			ctx,
			"/v5/market/instruments-info",
			params,
			&response,
		); err != nil {
			return nil, err
		}

		if response.RetCode != 0 {
			return nil, fmt.Errorf(
				"get instruments: %s",
				response.RetMsg,
			)
		}

		for _, item := range response.Result.List {

			// Нас интересуют только perpetual-контракты.
			if item.ContractType != "LinearPerpetual" {
				continue
			}

			launchTime, _ := strconv.ParseInt(
				item.LaunchTime,
				10,
				64,
			)

			deliveryTime, _ := strconv.ParseInt(
				item.DeliveryTime,
				10,
				64,
			)

			result = append(
				result,
				model.Instrument{
					Symbol:              item.Symbol,
					BaseCoin:            item.BaseCoin,
					QuoteCoin:           item.QuoteCoin,
					ContractType:        item.ContractType,
					Status:              item.Status,
					LaunchTime:          launchTime,
					DeliveryTime:        deliveryTime,
					FundingIntervalHour: item.FundingIntervalHour,
				},
			)
		}

		cursor = response.Result.NextPageCursor

		if cursor == "" {
			break
		}
	}

	return result, nil
}

// -----------------------------------------------------------------------------
// Tickers
// -----------------------------------------------------------------------------

type tickersResult struct {
	List []struct {
		Symbol string `json:"symbol"`

		LastPrice    string `json:"lastPrice"`
		Price24hPcnt string `json:"price24hPcnt"`
		Turnover24h  string `json:"turnover24h"`

		FundingRate string `json:"fundingRate"`

		OpenInterest      string `json:"openInterest"`
		OpenInterestValue string `json:"openInterestValue"`

		Bid1Price string `json:"bid1Price"`
		Ask1Price string `json:"ask1Price"`

		Bid1Size string `json:"bid1Size"`
		Ask1Size string `json:"ask1Size"`
	} `json:"list"`
}

// GetTickers получает тикеры всех linear-контрактов.
//
// Это намного эффективнее, чем делать:
// GET /ticker?symbol=BTCUSDT
// GET /ticker?symbol=ETHUSDT
// ...
//
// Один запрос отдаёт информацию по всем инструментам.
func (c *Client) GetTickers(
	ctx context.Context,
) (map[string]model.Ticker, error) {

	params := url.Values{}
	params.Set("category", category)

	var response apiResponse[tickersResult]

	if err := c.do(
		ctx,
		"/v5/market/tickers",
		params,
		&response,
	); err != nil {
		return nil, err
	}

	if response.RetCode != 0 {
		return nil, fmt.Errorf(
			"get tickers: %s",
			response.RetMsg,
		)
	}

	result := make(map[string]model.Ticker, len(response.Result.List))

	for _, item := range response.Result.List {

		lastPrice := parseFloat(item.LastPrice)
		change24h := parseFloat(item.Price24hPcnt) * 100
		turnover := parseFloat(item.Turnover24h)

		bid := parseFloat(item.Bid1Price)
		ask := parseFloat(item.Ask1Price)

		result[item.Symbol] = model.Ticker{
			Symbol:            item.Symbol,
			LastPrice:         lastPrice,
			Change24h:         change24h,
			Turnover24h:       turnover,
			FundingRate:       parseFloat(item.FundingRate),
			OpenInterest:      parseFloat(item.OpenInterest),
			OpenInterestValue: parseFloat(item.OpenInterestValue),
			BidPrice:          bid,
			AskPrice:          ask,
			BidSize:           parseFloat(item.Bid1Size),
			AskSize:           parseFloat(item.Ask1Size),
		}
	}

	return result, nil
}

// -----------------------------------------------------------------------------
// Klines
// -----------------------------------------------------------------------------

type klineResult struct {
	List [][]string `json:"list"`
}

// GetKlines получает исторические свечи.
//
// Bybit возвращает свечи в обратном порядке:
// самая новая свеча находится первой.
//
// Мы разворачиваем массив,
// чтобы внутри программы свечи шли от старых к новым.
func (c *Client) GetKlines(
	ctx context.Context,
	symbol string,
	interval string,
	limit int,
) ([]model.Candle, error) {

	params := url.Values{}

	params.Set("category", category)
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	params.Set("limit", strconv.Itoa(limit))

	var response apiResponse[klineResult]

	if err := c.do(
		ctx,
		"/v5/market/kline",
		params,
		&response,
	); err != nil {
		return nil, err
	}

	if response.RetCode != 0 {
		return nil, fmt.Errorf(
			"get klines %s %s: %s",
			symbol,
			interval,
			response.RetMsg,
		)
	}

	candles := make(
		[]model.Candle,
		0,
		len(response.Result.List),
	)

	for _, item := range response.Result.List {

		// Bybit candle:
		//
		// [0] timestamp
		// [1] open
		// [2] high
		// [3] low
		// [4] close
		// [5] volume
		//
		// [6] turnover
		//
		// Нас сейчас интересуют первые 6 значений.
		if len(item) < 6 {
			continue
		}

		timestamp, err := strconv.ParseInt(
			item[0],
			10,
			64,
		)

		if err != nil {
			continue
		}

		candles = append(
			candles,
			model.Candle{
				Time:   timestamp,
				Open:   parseFloat(item[1]),
				High:   parseFloat(item[2]),
				Low:    parseFloat(item[3]),
				Close:  parseFloat(item[4]),
				Volume: parseFloat(item[5]),
			},
		)
	}

	// Разворачиваем свечи.
	for left, right := 0, len(candles)-1; left < right; left, right = left+1, right-1 {
		candles[left], candles[right] =
			candles[right], candles[left]
	}

	return candles, nil
}

// -----------------------------------------------------------------------------
// Funding history
// -----------------------------------------------------------------------------

type fundingResult struct {
	List []struct {
		Symbol               string `json:"symbol"`
		FundingRate          string `json:"fundingRate"`
		FundingRateTimestamp string `json:"fundingRateTimestamp"`
	} `json:"list"`
}

// GetFundingHistory получает историю funding.
//
// Пока нам достаточно последних нескольких значений.
// Позже мы будем использовать их для анализа:
// funding растёт / падает / остаётся экстремальным.
func (c *Client) GetFundingHistory(
	ctx context.Context,
	symbol string,
	limit int,
) ([]float64, error) {

	params := url.Values{}

	params.Set("category", category)
	params.Set("symbol", symbol)
	params.Set("limit", strconv.Itoa(limit))

	var response apiResponse[fundingResult]

	if err := c.do(
		ctx,
		"/v5/market/funding/history",
		params,
		&response,
	); err != nil {
		return nil, err
	}

	if response.RetCode != 0 {
		return nil, fmt.Errorf(
			"get funding %s: %s",
			symbol,
			response.RetMsg,
		)
	}

	result := make(
		[]float64,
		0,
		len(response.Result.List),
	)

	for _, item := range response.Result.List {
		result = append(
			result,
			parseFloat(item.FundingRate),
		)
	}

	return result, nil
}

// -----------------------------------------------------------------------------
// Open Interest
// -----------------------------------------------------------------------------

type oiResult struct {
	List []struct {
		OpenInterest string `json:"openInterest"`
		Timestamp    string `json:"timestamp"`
	} `json:"list"`
}

// GetOpenInterest получает историю OI.
//
// Используем 1h данные.
func (c *Client) GetOpenInterest(
	ctx context.Context,
	symbol string,
	limit int,
) ([]float64, error) {

	params := url.Values{}

	params.Set("category", category)
	params.Set("symbol", symbol)
	params.Set("intervalTime", "1h")
	params.Set("limit", strconv.Itoa(limit))

	var response apiResponse[oiResult]

	if err := c.do(
		ctx,
		"/v5/market/open-interest",
		params,
		&response,
	); err != nil {
		return nil, err
	}

	if response.RetCode != 0 {
		return nil, fmt.Errorf(
			"get OI %s: %s",
			symbol,
			response.RetMsg,
		)
	}

	result := make(
		[]float64,
		0,
		len(response.Result.List),
	)

	for _, item := range response.Result.List {
		result = append(
			result,
			parseFloat(item.OpenInterest),
		)
	}

	return result, nil
}

// parseFloat — маленькая вспомогательная функция.
//
// API Bybit отдаёт практически все числовые значения как strings.
// Мы централизуем преобразование здесь.
func parseFloat(value string) float64 {
	if value == "" {
		return 0
	}

	result, err := strconv.ParseFloat(value, 64)

	if err != nil {
		return 0
	}

	return result
}
