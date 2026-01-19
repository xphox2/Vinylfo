package discogs

import (
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func logToFile(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	f, _ := os.OpenFile("sync_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg))
}

const (
	APIURL          = "https://api.discogs.com"
	AuthURL         = "https://api.discogs.com/oauth"
	AuthWebURL      = "https://www.discogs.com/oauth"
	RateLimitWindow = 60 * time.Second
	AuthRequests    = 60
	AnonRequests    = 25
)

type RateLimiter struct {
	sync.RWMutex
	windowStart   time.Time
	authRemaining int
	anonRemaining int
	lastAuthLimit int
	lastAnonLimit int
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		windowStart:   time.Now(),
		authRemaining: AuthRequests,
		anonRemaining: AnonRequests,
		lastAuthLimit: AuthRequests,
		lastAnonLimit: AnonRequests,
	}
}

func (rl *RateLimiter) Wait(isAuth bool) {
	rl.Lock()
	defer rl.Unlock()

	isAuthStr := "auth"
	if !isAuth {
		isAuthStr = "anon"
	}

	now := time.Now()
	elapsed := now.Sub(rl.windowStart)

	logToFile("RATELIMIT [%s]: Check at %s, elapsed=%v, window_start=%s",
		isAuthStr, now.Format("15:04:05.000"), elapsed, rl.windowStart.Format("15:04:05"))

	if elapsed >= RateLimitWindow {
		rl.windowStart = time.Now()
		if rl.lastAuthLimit > 0 {
			rl.authRemaining = rl.lastAuthLimit
		} else {
			rl.authRemaining = AuthRequests
		}
		if rl.lastAnonLimit > 0 {
			rl.anonRemaining = rl.lastAnonLimit
		} else {
			rl.anonRemaining = AnonRequests
		}
		logToFile("RATELIMIT [%s]: Window RESET, auth=%d, anon=%d",
			isAuthStr, rl.authRemaining, rl.anonRemaining)
	}

	remaining := rl.authRemaining
	if !isAuth {
		remaining = rl.anonRemaining
	}

	remainingThreshold := 5
	waitCount := 0
	for remaining <= remainingThreshold {
		waitCount++
		now := time.Now()
		sleepTime := rl.windowStart.Add(RateLimitWindow).Sub(now)
		if sleepTime > 0 {
			if rl.anonRemaining <= remainingThreshold && remaining > remainingThreshold {
				logToFile("RATELIMIT [%s]: Anonymous limit approaching (anon_rem=%d), sleeping %v until window reset (wait #%d)",
					isAuthStr, rl.anonRemaining, sleepTime, waitCount)
			} else {
				logToFile("RATELIMIT [%s]: Rate limit approaching (%s_rem=%d), sleeping %v until window reset (wait #%d)",
					isAuthStr, isAuthStr, remaining, sleepTime, waitCount)
			}
			time.Sleep(sleepTime)
		}
		rl.windowStart = time.Now()
		if rl.lastAuthLimit > 0 {
			rl.authRemaining = rl.lastAuthLimit
		} else {
			rl.authRemaining = AuthRequests
		}
		if rl.lastAnonLimit > 0 {
			rl.anonRemaining = rl.lastAnonLimit
		} else {
			rl.anonRemaining = AnonRequests
		}

		remaining = rl.authRemaining
		if !isAuth {
			remaining = rl.anonRemaining
		}
		logToFile("RATELIMIT [%s]: After wake - auth_rem=%d, anon_rem=%d",
			isAuthStr, rl.authRemaining, rl.anonRemaining)
	}

	logToFile("RATELIMIT [%s]: Proceeding with request, auth_rem=%d, anon_rem=%d",
		isAuthStr, rl.authRemaining, rl.anonRemaining)
}

func (rl *RateLimiter) GetDebugInfo() string {
	rl.RLock()
	defer rl.RUnlock()
	return fmt.Sprintf("auth_rem=%d, anon_rem=%d, window_elapsed=%.2fs",
		rl.authRemaining, rl.anonRemaining, time.Since(rl.windowStart).Seconds())
}

func (rl *RateLimiter) Decrement(isAuth bool) {
	rl.Lock()
	defer rl.Unlock()

	if isAuth {
		rl.authRemaining--
	} else {
		rl.anonRemaining--
	}

	logToFile("RATELIMIT: DECREMENTED - auth_rem=%d, anon_rem=%d",
		rl.authRemaining, rl.anonRemaining)
}

func (rl *RateLimiter) UpdateFromHeaders(resp *http.Response) {
	rl.Lock()
	defer rl.Unlock()

	rlAuth := resp.Header.Get("X-Discogs-Ratelimit-Auth")
	rlAuthRem := resp.Header.Get("X-Discogs-Ratelimit-Auth-Remaining")
	rlAnon := resp.Header.Get("X-Discogs-Ratelimit")
	rlAnonRem := resp.Header.Get("X-Discogs-Ratelimit-Remaining")

	logToFile("RATELIMIT HEADERS: Auth=%s/%s, Anon=%s/%s",
		rlAuth, rlAuthRem, rlAnon, rlAnonRem)

	authLimitSet := false
	if rlAuth != "" && rlAuth != "/" {
		if limit, err := strconv.Atoi(rlAuth); err == nil {
			rl.lastAuthLimit = limit
			authLimitSet = true
			if rlAuthRem != "" {
				if rem, err := strconv.Atoi(rlAuthRem); err == nil {
					rl.authRemaining = rem
					logToFile("RATELIMIT HEADERS: Updated auth_remaining=%d from header", rl.authRemaining)
				}
			} else {
				rl.authRemaining = limit
				logToFile("RATELIMIT HEADERS: No remaining header, set to limit=%d", rl.authRemaining)
			}
		}
	}

	if !authLimitSet && rl.lastAuthLimit == 0 {
		rl.lastAuthLimit = AuthRequests
		if rl.authRemaining == 0 {
			rl.authRemaining = AuthRequests
		}
	}

	if rlAnon != "" {
		if limit, err := strconv.Atoi(rlAnon); err == nil {
			rl.lastAnonLimit = limit
			if rlAnonRem != "" {
				if rem, err := strconv.Atoi(rlAnonRem); err == nil {
					rl.anonRemaining = rem
				} else {
					rl.anonRemaining = limit
				}
			} else {
				rl.anonRemaining = limit
			}
		}
	} else if rlAnonRem != "" && rl.lastAnonLimit == 0 {
		if rem, err := strconv.Atoi(rlAnonRem); err == nil {
			rl.anonRemaining = rem
		}
	}
}

func (rl *RateLimiter) WaitForReset(retryAfter int) {
	rl.Lock()
	defer rl.Unlock()

	sleepTime := time.Duration(retryAfter) * time.Second
	if sleepTime <= 0 {
		sleepTime = RateLimitWindow
	}
	logToFile("RATELIMIT: Waiting %v for rate limit reset (Retry-After: %ds)", sleepTime, retryAfter)
	time.Sleep(sleepTime)
	rl.windowStart = time.Now()
	rl.authRemaining = rl.lastAuthLimit
	rl.anonRemaining = rl.lastAnonLimit
}

func (rl *RateLimiter) GetRemaining() int {
	rl.RLock()
	defer rl.RUnlock()
	return rl.authRemaining
}

func (rl *RateLimiter) GetRemainingAnon() int {
	rl.RLock()
	defer rl.RUnlock()
	return rl.anonRemaining
}

type OAuthConfig struct {
	ConsumerKey    string
	ConsumerSecret string
	AccessToken    string
	AccessSecret   string
}

type Client struct {
	HTTPClient  *http.Client
	RateLimiter *RateLimiter
	OAuth       *OAuthConfig
	APIKey      string
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:      apiKey,
		HTTPClient:  &http.Client{Timeout: 60 * time.Second},
		RateLimiter: NewRateLimiter(),
	}
}

func NewClientWithOAuth(apiKey string, oauth *OAuthConfig) *Client {
	client := &Client{
		APIKey:      apiKey,
		HTTPClient:  &http.Client{Timeout: 60 * time.Second},
		RateLimiter: NewRateLimiter(),
		OAuth:       oauth,
	}
	return client
}

func (c *Client) GetAPIRemaining() int {
	return c.RateLimiter.GetRemaining()
}

func (c *Client) GetAPIRemainingAnon() int {
	return c.RateLimiter.GetRemainingAnon()
}

func loadOAuthConfig() *OAuthConfig {
	return &OAuthConfig{
		ConsumerKey:    os.Getenv("DISCOGS_CONSUMER_KEY"),
		ConsumerSecret: os.Getenv("DISCOGS_CONSUMER_SECRET"),
		AccessToken:    os.Getenv("DISCOGS_ACCESS_TOKEN"),
		AccessSecret:   os.Getenv("DISCOGS_ACCESS_SECRET"),
	}
}

func (c *Client) IsAuthenticated() bool {
	return c.OAuth != nil && c.OAuth.AccessToken != "" && c.OAuth.AccessSecret != ""
}

func (c *Client) makeRequest(method, requestURL string, body url.Values) (*http.Response, error) {
	isAuth := c.APIKey != ""
	logToFile("API REQUEST [%s]: %s %s", map[bool]string{true: "auth", false: "anon"}[isAuth], method, requestURL)

	c.RateLimiter.Wait(isAuth)

	req, err := http.NewRequest(method, requestURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Vinylfo/1.0 (https://github.com/xphox2/Vinylfo)")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Discogs token=%s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	c.RateLimiter.UpdateFromHeaders(resp)

	if resp.StatusCode == http.StatusTooManyRequests {
		io.ReadAll(resp.Body)
		resp.Body.Close()

		retryAfter := 60
		if retryHeader := resp.Header.Get("Retry-After"); retryHeader != "" {
			if seconds, err := strconv.Atoi(retryHeader); err == nil && seconds > 0 {
				retryAfter = seconds
			}
		}

		logToFile("API ERROR 429: Retry-After=%ds, RateLimit-Auth=%s, RateLimit-Auth-Remaining=%s",
			retryAfter,
			resp.Header.Get("X-Discogs-Ratelimit-Auth"),
			resp.Header.Get("X-Discogs-Ratelimit-Auth-Remaining"))

		c.RateLimiter.WaitForReset(retryAfter)
		return c.makeRequest(method, requestURL, body)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Discogs API error: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	c.RateLimiter.Decrement(isAuth)

	logToFile("API SUCCESS: %s %s -> %d", method, requestURL, resp.StatusCode)
	return resp, nil
}

func (c *Client) makeOAuthRequest(method, requestURL string, body url.Values) (*http.Response, error) {
	logToFile("makeOAuthRequest: starting for %s %s", method, requestURL)

	if c.OAuth == nil {
		return nil, fmt.Errorf("makeOAuthRequest: OAuth config is nil")
	}
	if c.OAuth.ConsumerKey == "" {
		return nil, fmt.Errorf("makeOAuthRequest: OAuth ConsumerKey is empty")
	}
	if c.OAuth.AccessToken == "" {
		return nil, fmt.Errorf("makeOAuthRequest: OAuth AccessToken is empty")
	}

	c.RateLimiter.Wait(true)

	if body == nil {
		body = url.Values{}
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	oauthParams := map[string]string{
		"oauth_consumer_key":     c.OAuth.ConsumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            c.OAuth.AccessToken,
		"oauth_version":          "1.0",
	}

	params := make(url.Values)
	for k, v := range oauthParams {
		params.Set(k, v)
	}

	for k, v := range body {
		params[k] = v
	}

	if parsedURL, err := url.Parse(requestURL); err == nil {
		for k, v := range parsedURL.Query() {
			for _, vv := range v {
				params.Set(k, vv)
			}
		}
	}

	var paramKeys []string
	for k := range params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)

	var paramPairs []string
	for _, k := range paramKeys {
		v := params.Get(k)
		v = strings.ReplaceAll(v, " ", "%20")
		paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", percentEncode(k), percentEncodeValue(v)))
	}

	baseURL := requestURL
	if parsedURL, err := url.Parse(requestURL); err == nil {
		baseURL = fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)
	}

	baseString := fmt.Sprintf("%s&%s&%s", method, url.QueryEscape(baseURL), url.QueryEscape(strings.Join(paramPairs, "&")))

	logToFile("makeOAuthRequest: baseURL=%s", baseURL)
	logToFile("makeOAuthRequest: baseString=%s", baseString)
	logToFile("makeOAuthRequest: ConsumerSecret=%s, AccessSecret=%s", maskValue(c.OAuth.ConsumerSecret), maskValue(c.OAuth.AccessSecret))

	signature := generateHmacSignature(baseString, c.OAuth.ConsumerSecret, c.OAuth.AccessSecret)

	logToFile("makeOAuthRequest: signature=%s", signature)

	authHeader := fmt.Sprintf(`OAuth oauth_consumer_key="%s", oauth_token="%s", oauth_signature_method="HMAC-SHA1", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_signature="%s"`,
		url.QueryEscape(c.OAuth.ConsumerKey),
		url.QueryEscape(c.OAuth.AccessToken),
		timestamp,
		nonce,
		signature,
	)

	req, err := http.NewRequest(method, requestURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", "\"Windows\"")

	resp, err := c.HTTPClient.Do(req)
	logToFile("makeOAuthRequest: HTTP response received, err=%v", err)
	if err != nil {
		return nil, err
	}

	c.RateLimiter.UpdateFromHeaders(resp)

	if resp.StatusCode == http.StatusTooManyRequests {
		io.ReadAll(resp.Body)
		resp.Body.Close()

		retryAfter := 60
		if retryHeader := resp.Header.Get("Retry-After"); retryHeader != "" {
			if seconds, err := strconv.Atoi(retryHeader); err == nil && seconds > 0 {
				retryAfter = seconds
			}
		}

		logToFile("API ERROR 429: Retry-After=%ds, RateLimit-Auth=%s, RateLimit-Auth-Remaining=%s",
			retryAfter,
			resp.Header.Get("X-Discogs-Ratelimit-Auth"),
			resp.Header.Get("X-Discogs-Ratelimit-Auth-Remaining"))

		c.RateLimiter.WaitForReset(retryAfter)
		return c.makeOAuthRequest(method, requestURL, nil)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		logToFile("makeOAuthRequest: API error %d - %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("Discogs API error: %d - %s", resp.StatusCode, string(body))
	}

	c.RateLimiter.Decrement(true)

	logToFile("makeOAuthRequest: Success! Content-Encoding=%s", resp.Header.Get("Content-Encoding"))

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	encoding := resp.Header.Get("Content-Encoding")
	if encoding == "gzip" {
		gzReader, _ := gzip.NewReader(strings.NewReader(string(bodyBytes)))
		bodyBytes, _ = io.ReadAll(gzReader)
		gzReader.Close()
	}

	resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	return resp, nil
}

func (c *Client) makeAuthenticatedRequest(method, requestURL string, body url.Values) (*http.Response, error) {
	if c.OAuth != nil {
		return c.makeOAuthRequest(method, requestURL, body)
	}
	return c.makeRequest(method, requestURL, body)
}

func generateHmacSignature(baseString, consumerSecret, tokenSecret string) string {
	key := fmt.Sprintf("%s&%s", url.QueryEscape(consumerSecret), url.QueryEscape(tokenSecret))

	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

func generatePlainTextSignature(consumerSecret, tokenSecret string) string {
	consumerSecret = strings.TrimSpace(consumerSecret)
	tokenSecret = strings.TrimSpace(tokenSecret)
	return fmt.Sprintf("%s&%s", consumerSecret, tokenSecret)
}

func (c *Client) GetRequestToken() (token, secret, authURL string, err error) {
	consumerKey := os.Getenv("DISCOGS_CONSUMER_KEY")
	consumerSecret := os.Getenv("DISCOGS_CONSUMER_SECRET")
	callbackURL := os.Getenv("DISCOGS_CALLBACK_URL")

	if consumerKey == "" || consumerSecret == "" {
		return "", "", "", fmt.Errorf("DISCOGS_CONSUMER_KEY or DISCOGS_CONSUMER_SECRET not set")
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	oauthParams := fmt.Sprintf("oauth_callback=%s&oauth_consumer_key=%s&oauth_nonce=%s&oauth_signature_method=HMAC-SHA1&oauth_timestamp=%s&oauth_version=1.0",
		url.QueryEscape(callbackURL),
		url.QueryEscape(consumerKey),
		nonce,
		timestamp)

	baseString := fmt.Sprintf("%s&%s&%s", "POST", url.QueryEscape(AuthURL+"/request_token"), url.QueryEscape(oauthParams))

	signature := generateHmacSignature(baseString, consumerSecret, "")

	authHeader := fmt.Sprintf(`OAuth oauth_consumer_key="%s", oauth_signature="%s", oauth_signature_method="HMAC-SHA1", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_callback="%s"`,
		url.QueryEscape(consumerKey),
		url.QueryEscape(signature),
		timestamp,
		nonce,
		url.QueryEscape(callbackURL))

	req, err := http.NewRequest("POST", AuthURL+"/request_token", nil)
	if err != nil {
		return "", "", "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", "\"Windows\"")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", "", "", fmt.Errorf("Discogs OAuth request failed with status %d. This is likely due to Cloudflare protection blocking automated requests. Please complete OAuth manually through a web browser.", resp.StatusCode)
	}

	values, _ := url.ParseQuery(string(body))

	token = values.Get("oauth_token")
	secret = values.Get("oauth_token_secret")
	authURL = AuthWebURL + "/authorize?oauth_token=" + token

	if token == "" {
		return "", "", "", fmt.Errorf("no oauth_token in response")
	}

	return token, secret, authURL, nil
}

func (c *Client) GetAccessToken(token, secret, verifier string) (accessToken, accessSecret, username string, err error) {
	c.OAuth.ConsumerKey = os.Getenv("DISCOGS_CONSUMER_KEY")
	c.OAuth.ConsumerSecret = os.Getenv("DISCOGS_CONSUMER_SECRET")
	c.OAuth.AccessToken = token
	c.OAuth.AccessSecret = secret

	if c.OAuth.ConsumerKey == "" || c.OAuth.ConsumerSecret == "" {
		return "", "", "", fmt.Errorf("DISCOGS_CONSUMER_KEY or DISCOGS_CONSUMER_SECRET not set")
	}

	data := url.Values{}
	data.Set("oauth_token", token)
	data.Set("oauth_verifier", verifier)

	resp, err := c.makeOAuthRequest("POST", AuthURL+"/access_token", data)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	values, _ := url.ParseQuery(string(body))

	accessToken = values.Get("oauth_token")
	accessSecret = values.Get("oauth_token_secret")
	username = values.Get("username")

	return accessToken, accessSecret, username, nil
}

func (c *Client) GetUserIdentity() (username string, err error) {
	resp, err := c.makeOAuthRequest("GET", APIURL+"/oauth/identity", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var identity struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		return "", err
	}

	return identity.Username, nil
}

func (c *Client) GetUserCollection(username string, page, perPage int) ([]map[string]interface{}, error) {
	if username == "" {
		return nil, fmt.Errorf("GetUserCollection: username is empty")
	}

	if c.OAuth == nil {
		return nil, fmt.Errorf("GetUserCollection: OAuth is nil")
	}
	if c.OAuth.ConsumerKey == "" {
		return nil, fmt.Errorf("GetUserCollection: ConsumerKey is empty")
	}
	if c.OAuth.AccessToken == "" {
		return nil, fmt.Errorf("GetUserCollection: AccessToken is empty")
	}

	requestURL := fmt.Sprintf("%s/users/%s/collection/folders/0/releases?page=%d&per_page=%d",
		APIURL, url.QueryEscape(username), page, perPage)

	logToFile("DISCOGS_API: GET %s", requestURL)
	logToFile("DISCOGS_API: Auth - ConsumerKey=%s, AccessToken=%s", maskValue(c.OAuth.ConsumerKey), maskValue(c.OAuth.AccessToken))

	resp, err := c.makeOAuthRequest("GET", requestURL, nil)
	if err != nil {
		logToFile("DISCOGS_API: ERROR - %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var collection struct {
		Releases []struct {
			ID               int    `json:"id"`
			InstanceID       int    `json:"instance_id"`
			DateAdded        string `json:"date_added"`
			BasicInformation struct {
				Title   string `json:"title"`
				Year    int    `json:"year"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
				Images []struct {
					URI string `json:"uri"`
				} `json:"images"`
			} `json:"basic_information"`
		} `json:"releases"`
		Pagination struct {
			Page    int `json:"page"`
			Pages   int `json:"pages"`
			PerPage int `json:"per_page"`
			Items   int `json:"items"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, err
	}

	releases := make([]map[string]interface{}, 0)
	for _, r := range collection.Releases {
		artistName := ""
		if len(r.BasicInformation.Artists) > 0 {
			artistName = r.BasicInformation.Artists[0].Name
		}

		coverImage := ""
		if len(r.BasicInformation.Images) > 0 {
			coverImage = r.BasicInformation.Images[0].URI
		}

		releases = append(releases, map[string]interface{}{
			"discogs_id":  r.ID,
			"instance_id": r.InstanceID,
			"title":       r.BasicInformation.Title,
			"artist":      artistName,
			"year":        r.BasicInformation.Year,
			"cover_image": coverImage,
			"date_added":  r.DateAdded,
			"folder_id":   0,
		})
	}

	logToFile("DISCOGS_API: Success! Got %d releases", len(releases))
	return releases, nil
}

func (c *Client) GetUserCollectionByFolder(username string, folderID int, page, perPage int) ([]map[string]interface{}, int, error) {
	if username == "" {
		return nil, 0, fmt.Errorf("GetUserCollectionByFolder: username is empty")
	}

	if c.OAuth == nil {
		return nil, 0, fmt.Errorf("GetUserCollectionByFolder: OAuth is nil")
	}
	if c.OAuth.ConsumerKey == "" {
		return nil, 0, fmt.Errorf("GetUserCollectionByFolder: ConsumerKey is empty")
	}
	if c.OAuth.AccessToken == "" {
		return nil, 0, fmt.Errorf("GetUserCollectionByFolder: AccessToken is empty")
	}

	requestURL := fmt.Sprintf("%s/users/%s/collection/folders/%d/releases?page=%d&per_page=%d",
		APIURL, url.QueryEscape(username), folderID, page, perPage)

	logToFile("DISCOGS_API: GET %s", requestURL)
	logToFile("DISCOGS_API: Auth - ConsumerKey=%s, AccessToken=%s", maskValue(c.OAuth.ConsumerKey), maskValue(c.OAuth.AccessToken))

	resp, err := c.makeOAuthRequest("GET", requestURL, nil)
	if err != nil {
		logToFile("DISCOGS_API: ERROR - %v", err)
		return nil, 0, err
	}
	defer resp.Body.Close()

	var collection struct {
		Releases []struct {
			ID               int    `json:"id"`
			InstanceID       int    `json:"instance_id"`
			DateAdded        string `json:"date_added"`
			BasicInformation struct {
				Title   string `json:"title"`
				Year    int    `json:"year"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
				Images []struct {
					URI string `json:"uri"`
				} `json:"images"`
			} `json:"basic_information"`
		} `json:"releases"`
		Pagination struct {
			Page    int `json:"page"`
			Pages   int `json:"pages"`
			PerPage int `json:"per_page"`
			Items   int `json:"items"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, 0, err
	}

	releases := make([]map[string]interface{}, 0)
	for _, r := range collection.Releases {
		artistName := ""
		if len(r.BasicInformation.Artists) > 0 {
			artistName = r.BasicInformation.Artists[0].Name
		}

		coverImage := ""
		if len(r.BasicInformation.Images) > 0 {
			coverImage = r.BasicInformation.Images[0].URI
		}

		releases = append(releases, map[string]interface{}{
			"discogs_id":  r.ID,
			"instance_id": r.InstanceID,
			"title":       r.BasicInformation.Title,
			"artist":      artistName,
			"year":        r.BasicInformation.Year,
			"cover_image": coverImage,
			"date_added":  r.DateAdded,
			"folder_id":   folderID,
		})
	}

	logToFile("DISCOGS_API: Success! Got %d releases from folder %d (total items: %d)", len(releases), folderID, collection.Pagination.Items)
	return releases, collection.Pagination.Items, nil
}

func (c *Client) GetUserFolders(username string) ([]map[string]interface{}, error) {
	if username == "" {
		return nil, fmt.Errorf("GetUserFolders: username is empty")
	}

	if c.OAuth == nil {
		return nil, fmt.Errorf("GetUserFolders: OAuth is nil")
	}
	if c.OAuth.ConsumerKey == "" {
		return nil, fmt.Errorf("GetUserFolders: ConsumerKey is empty")
	}
	if c.OAuth.AccessToken == "" {
		return nil, fmt.Errorf("GetUserFolders: AccessToken is empty")
	}

	requestURL := fmt.Sprintf("%s/users/%s/collection/folders", APIURL, url.QueryEscape(username))

	logToFile("DISCOGS_API: GET %s", requestURL)
	logToFile("DISCOGS_API: Auth - ConsumerKey=%s, AccessToken=%s", maskValue(c.OAuth.ConsumerKey), maskValue(c.OAuth.AccessToken))

	resp, err := c.makeOAuthRequest("GET", requestURL, nil)
	if err != nil {
		logToFile("DISCOGS_API: ERROR - %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var foldersResponse struct {
		Folders []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Count       int    `json:"count"`
			ResourceURL string `json:"resource_url"`
		} `json:"folders"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&foldersResponse); err != nil {
		return nil, err
	}

	folders := make([]map[string]interface{}, 0)
	for _, f := range foldersResponse.Folders {
		folders = append(folders, map[string]interface{}{
			"id":           f.ID,
			"name":         f.Name,
			"count":        f.Count,
			"resource_url": f.ResourceURL,
		})
	}

	logToFile("DISCOGS_API: Success! Got %d folders", len(folders))
	return folders, nil
}

func (c *Client) SearchAlbums(query string, page int) ([]map[string]interface{}, int, error) {
	searchURL := fmt.Sprintf("%s/database/search?q=%s&type=release&page=%d&per_page=12&sort=year&sort_order=desc",
		APIURL, strings.ReplaceAll(url.QueryEscape(query), "+", "%20"), page)

	resp, err := c.makeAuthenticatedRequest("GET", searchURL, nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var searchResponse struct {
		Results []struct {
			ID      int    `json:"id"`
			Title   string `json:"title"`
			Year    any    `json:"year"`
			Country string `json:"country"`
			Format  any    `json:"format"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			CoverImage string `json:"cover_image"`
		} `json:"results"`
		Pagination struct {
			Page  int `json:"page"`
			Pages int `json:"pages"`
			Items int `json:"items"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, 0, err
	}

	albums := make([]map[string]interface{}, 0)
	for _, result := range searchResponse.Results {
		artistName := ""
		if len(result.Artists) > 0 {
			artistName = result.Artists[0].Name
		}

		year := 0
		switch v := result.Year.(type) {
		case float64:
			year = int(v)
		case string:
			if v != "" {
				year, _ = strconv.Atoi(v)
			}
		}

		format := ""
		switch v := result.Format.(type) {
		case string:
			format = v
		case []interface{}:
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					format = s
				}
			}
		}

		albums = append(albums, map[string]interface{}{
			"discogs_id":  result.ID,
			"title":       result.Title,
			"artist":      artistName,
			"year":        year,
			"country":     result.Country,
			"format":      format,
			"cover_image": result.CoverImage,
		})
	}

	return albums, searchResponse.Pagination.Pages, nil
}

func (c *Client) GetAlbum(id int) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/releases/%d", APIURL, id)
	resp, err := c.makeAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	album, err := parseAlbumResponse(resp)
	if err != nil {
		return nil, err
	}

	return album, nil
}

// GetMasterRelease gets the master release for an album
// Master releases are the main entry point and always public/stable
func (c *Client) GetMasterRelease(id int) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/masters/%d", APIURL, id)
	resp, err := c.makeAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var master struct {
		ID      int    `json:"id"`
		Title   string `json:"title"`
		Year    int    `json:"year"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		Images []struct {
			URI  string `json:"uri"`
			Type string `json:"type"`
		} `json:"images"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&master); err != nil {
		return nil, err
	}

	artistName := ""
	if len(master.Artists) > 0 {
		artistName = master.Artists[0].Name
	}

	return map[string]interface{}{
		"id":          master.ID,
		"title":       master.Title,
		"artist":      artistName,
		"year":        master.Year,
		"cover_image": "",
		"is_master":   true,
	}, nil
}

// GetMainReleaseFromMaster gets the most popular release from a master
// This returns a public release with full tracklist and durations
func (c *Client) GetMainReleaseFromMaster(masterID int) (int, error) {
	url := fmt.Sprintf("%s/masters/%d/releases?per_page=1&sort=year&sort_order=desc", APIURL, masterID)
	resp, err := c.makeAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var respData struct {
		Releases []struct {
			ID int `json:"id"`
		} `json:"releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return 0, err
	}

	if len(respData.Releases) > 0 {
		return respData.Releases[0].ID, nil
	}

	return 0, fmt.Errorf("no releases found for master %d", masterID)
}

func parseAlbumResponse(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()

	var discogsAlbum struct {
		ID       int      `json:"id"`
		Title    string   `json:"title"`
		Year     int      `json:"year"`
		Country  string   `json:"country"`
		Label    string   `json:"label"`
		Released string   `json:"released"`
		Genres   []string `json:"genres"`
		Styles   []string `json:"styles"`
		Images   []struct {
			URI  string `json:"uri"`
			Type string `json:"type"`
		} `json:"images"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		Tracklist []struct {
			Title       string `json:"title"`
			Duration    string `json:"duration"`
			Position    string `json:"position"`
			TrackNumber string `json:"track_number"`
			DiscNumber  string `json:"disc_number"`
		} `json:"tracklist"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&discogsAlbum); err != nil {
		return nil, err
	}

	artistName := ""
	if len(discogsAlbum.Artists) > 0 {
		artistName = discogsAlbum.Artists[0].Name
	}

	coverImage := ""
	for _, img := range discogsAlbum.Images {
		if img.Type == "primary" {
			coverImage = img.URI
			break
		}
	}
	if coverImage == "" && len(discogsAlbum.Images) > 0 {
		coverImage = discogsAlbum.Images[0].URI
	}

	genre := ""
	if len(discogsAlbum.Genres) > 0 {
		genre = discogsAlbum.Genres[0]
	}

	styles := ""
	if len(discogsAlbum.Styles) > 0 {
		styles = strings.Join(discogsAlbum.Styles, ", ")
	}

	album := map[string]interface{}{
		"discogs_id":   discogsAlbum.ID,
		"title":        discogsAlbum.Title,
		"artist":       artistName,
		"year":         discogsAlbum.Year,
		"country":      discogsAlbum.Country,
		"label":        discogsAlbum.Label,
		"release_date": discogsAlbum.Released,
		"genre":        genre,
		"style":        styles,
		"cover_image":  coverImage,
		"tracklist":    parseTracklist(discogsAlbum.Tracklist),
	}

	return album, nil
}

func parseTracklist(tracklist []struct {
	Title       string `json:"title"`
	Duration    string `json:"duration"`
	Position    string `json:"position"`
	TrackNumber string `json:"track_number"`
	DiscNumber  string `json:"disc_number"`
}) []map[string]interface{} {
	tracks := make([]map[string]interface{}, 0)

	logToFile("parseTracklist: processing %d tracks", len(tracklist))

	positionInfos := make([]PositionInfo, 0, len(tracklist))
	for _, track := range tracklist {
		posInfo := ParsePosition(track.Position)
		positionInfos = append(positionInfos, posInfo)
		logToFile("parseTracklist: raw_position=%s -> standard=%s, disc=%d, track=%d, side=%s, valid=%v",
			track.Position, convertPositionToStandard(track.Position),
			posInfo.DiscNumber, posInfo.TrackNumber, posInfo.Side, posInfo.IsValid)
	}

	trackCounter := 0
	for i, track := range tracklist {
		posInfo := positionInfos[i]
		side := convertPositionToStandard(track.Position)

		discNumber := 0
		trackNumber := 0

		if track.TrackNumber != "" {
			if n, err := strconv.Atoi(track.TrackNumber); err == nil {
				trackNumber = n
			} else {
				trackCounter++
				trackNumber = trackCounter
			}
		} else {
			trackCounter++
			trackNumber = trackCounter
		}

		if track.DiscNumber != "" {
			if n, err := strconv.Atoi(track.DiscNumber); err == nil {
				discNumber = n
			} else if posInfo.IsValid {
				discNumber = posInfo.DiscNumber
			} else {
				discNumber = 1
			}
		} else if posInfo.IsValid {
			discNumber = posInfo.DiscNumber
		} else {
			discNumber = 1
		}

		logToFile("parseTracklist: track=%s, position=%s -> disc_number=%d, track_number=%d",
			track.Title, side, discNumber, trackNumber)

		tracks = append(tracks, map[string]interface{}{
			"track_number": trackNumber,
			"disc_number":  discNumber,
			"position":     side,
			"title":        track.Title,
			"duration":     durationToSeconds(track.Duration),
		})
	}
	return tracks
}

func convertPositionToStandard(position string) string {
	if position == "" {
		return ""
	}

	position = strings.TrimSpace(position)

	if len(position) >= 2 {
		firstChar := position[0]
		if firstChar >= 'A' && firstChar <= 'Z' {
			return position
		}
	}

	parts := strings.Split(position, "-")
	if len(parts) == 2 {
		discNum, err1 := strconv.Atoi(parts[0])
		trackNum := parts[1]
		if err1 == nil && discNum > 0 {
			discLetter := string(rune('A' + discNum - 1))
			return fmt.Sprintf("%s%s", discLetter, trackNum)
		}
	}

	if len(position) >= 2 {
		firstChar := position[0]
		if firstChar >= '0' && firstChar <= '9' {
			for i := 1; i < len(position); i++ {
				if position[i] >= '0' && position[i] <= '9' {
					discPart := position[:i]
					trackPart := position[i:]
					discNum, err1 := strconv.Atoi(discPart)
					if err1 == nil && discNum > 0 {
						discLetter := string(rune('A' + discNum - 1))
						return fmt.Sprintf("%s%s", discLetter, trackPart)
					}
					break
				}
			}
		}
	}

	return position
}

func durationToSeconds(duration string) int {
	if duration == "" {
		return 0
	}

	parts := strings.Split(duration, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}

	var totalSeconds int
	for _, part := range parts {
		seconds, err := strconv.Atoi(part)
		if err != nil {
			return 0
		}
		totalSeconds = totalSeconds*60 + seconds
	}

	return totalSeconds
}

type PositionInfo struct {
	DiscNumber  int
	TrackNumber int
	Side        string
	SideNumber  int
	IsValid     bool
}

func ParsePosition(position string) PositionInfo {
	if position == "" {
		return PositionInfo{IsValid: false}
	}

	position = strings.TrimSpace(position)
	if position == "" {
		return PositionInfo{IsValid: false}
	}

	standardPos := convertPositionToStandard(position)
	if standardPos == "" {
		return PositionInfo{IsValid: false}
	}

	firstChar := standardPos[0]
	if firstChar < 'A' || firstChar > 'Z' {
		return PositionInfo{IsValid: false}
	}

	side := string(firstChar)
	discNumber := 0
	sideNumber := 0

	switch firstChar {
	case 'A':
		discNumber = 1
		sideNumber = 1
	case 'B':
		discNumber = 1
		sideNumber = 2
	case 'C':
		discNumber = 2
		sideNumber = 1
	case 'D':
		discNumber = 2
		sideNumber = 2
	case 'E':
		discNumber = 3
		sideNumber = 1
	case 'F':
		discNumber = 3
		sideNumber = 2
	default:
		discNumber = 1
		sideNumber = 1
	}

	trackNumStr := standardPos[1:]
	trackNum, err := strconv.Atoi(trackNumStr)
	if err != nil || trackNum < 0 {
		trackNum = 0
	}

	return PositionInfo{
		DiscNumber:  discNumber,
		TrackNumber: trackNum,
		Side:        side,
		SideNumber:  sideNumber,
		IsValid:     true,
	}
}

func maskValue(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

func (c *Client) GetTracksForAlbum(id int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/releases/%d", APIURL, id)
	resp, err := c.makeAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	album, err := parseAlbumResponse(resp)
	if err != nil {
		return nil, err
	}

	return album["tracklist"].([]map[string]interface{}), nil
}

func (c *Client) CrossReferenceTimestamps(title, artist string, currentTracks []map[string]interface{}) ([]map[string]interface{}, error) {
	hasDurations := false
	for _, track := range currentTracks {
		if dur, ok := track["duration"].(int); ok && dur > 0 {
			hasDurations = true
			break
		}
	}

	if hasDurations {
		logToFile("CrossReferenceTimestamps: Skipping - tracks already have durations for %s - %s", artist, title)
		return currentTracks, nil
	}

	logToFile("CrossReferenceTimestamps: No durations found for %s - %s, searching for alternative releases", artist, title)

	var allResults []map[string]interface{}

	for page := 1; page <= 3; page++ {
		searchQuery := fmt.Sprintf("%s %s", artist, title)
		searchQuery = strings.ReplaceAll(searchQuery, "/", " ")
		logToFile("CrossReferenceTimestamps: Searching page %d with query: %q", page, searchQuery)
		searchResults, _, err := c.SearchAlbums(searchQuery, page)
		if err != nil {
			logToFile("CrossReferenceTimestamps: Search failed for page %d: %v", page, err)
			break
		}

		if len(searchResults) == 0 {
			break
		}

		logToFile("CrossReferenceTimestamps: Page %d returned %d results", page, len(searchResults))
		allResults = append(allResults, searchResults...)

		if len(searchResults) < 12 {
			break
		}
	}

	logToFile("CrossReferenceTimestamps: Total results collected: %d", len(allResults))

	if len(allResults) == 0 {
		logToFile("CrossReferenceTimestamps: No search results for %s - %s", artist, title)
		return currentTracks, nil
	}

	normalizedTitle := normalizeStringForCompare(title)
	normalizedArtist := normalizeStringForCompare(artist)

	for i, result := range allResults {
		resultTitle, _ := result["title"].(string)
		resultArtist, _ := result["artist"].(string)
		resultID, _ := result["discogs_id"].(int)

		logToFile("CrossReferenceTimestamps: Result %d: id=%d, title=%q, artist=%q", i, resultID, resultTitle, resultArtist)

		if resultArtist == "" && strings.Contains(resultTitle, "-") {
			cleanTitle := removeZeroWidthChars(resultTitle)
			if cleanTitle != resultTitle {
				resultTitle = cleanTitle
				logToFile("CrossReferenceTimestamps: Cleaned unicode from title: %q", resultTitle)
			}
			parts := strings.SplitN(resultTitle, "-", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
				resultArtist = strings.TrimSpace(parts[0])
				resultTitle = strings.TrimSpace(parts[1])
				logToFile("CrossReferenceTimestamps: Extracted from title format: artist=%q, title=%q", resultArtist, resultTitle)
			}
		}

		normalizedResultTitle := normalizeStringForCompare(resultTitle)
		normalizedResultArtist := normalizeStringForCompare(resultArtist)

		titleScore := stringSimilarity(normalizedResultTitle, normalizedTitle)
		artistScore := stringSimilarity(normalizedResultArtist, normalizedArtist)

		logToFile("CrossReferenceTimestamps: Similarity scores - title: %.2f (%q vs %q), artist: %.2f (%q vs %q)",
			titleScore, normalizedResultTitle, normalizedTitle, artistScore, normalizedResultArtist, normalizedArtist)

		isSameTitle := normalizedResultTitle == normalizedTitle
		isSameArtist := normalizedResultArtist == normalizedArtist ||
			strings.Contains(normalizedResultArtist, normalizedArtist) ||
			strings.Contains(normalizedArtist, normalizedResultArtist)
		hasArtistInResult := normalizedResultArtist != ""
		hasArtistInSearch := normalizedArtist != ""

		if resultID == 0 {
			logToFile("CrossReferenceTimestamps: Skipping result %d - no discogs_id", i)
			continue
		}

		isExactMatch := isSameTitle && (isSameArtist || (!hasArtistInResult && !hasArtistInSearch))
		isHighSimilarityMatch := titleScore >= 0.80 && (artistScore >= 0.80 || (!hasArtistInResult && hasArtistInSearch))

		logToFile("CrossReferenceTimestamps: Match evaluation - titleScore=%.2f(>=0.80:%v), artistScore=%.2f(>=0.80:%v), hasArtistInResult=%v, hasArtistInSearch=%v",
			titleScore, titleScore >= 0.80, artistScore, artistScore >= 0.80, hasArtistInResult, hasArtistInSearch)
		logToFile("CrossReferenceTimestamps: isExactMatch=%v, isHighSimilarityMatch=%v", isExactMatch, isHighSimilarityMatch)

		if isExactMatch || isHighSimilarityMatch {
			matchType := "EXACT"
			if isHighSimilarityMatch && !isExactMatch {
				matchType = "HIGH SIMILARITY"
			}
			logToFile("CrossReferenceTimestamps: Found %s MATCH release %d", matchType, resultID)

			altTracks, err := c.GetTracksForAlbum(resultID)
			if err != nil {
				logToFile("CrossReferenceTimestamps: Failed to fetch tracks for release %d: %v", resultID, err)
				continue
			}

			logToFile("CrossReferenceTimestamps: Fetched %d tracks from release %d", len(altTracks), resultID)

			altHasDurations := false
			for j, track := range altTracks {
				dur, _ := track["duration"].(int)
				trackTitle, _ := track["title"].(string)
				if dur > 0 {
					altHasDurations = true
					logToFile("CrossReferenceTimestamps: Alt track %d: %q duration=%d (HAS DATA)", j, trackTitle, dur)
				} else {
					logToFile("CrossReferenceTimestamps: Alt track %d: %q duration=%d", j, trackTitle, dur)
				}
			}

			if !altHasDurations {
				logToFile("CrossReferenceTimestamps: Alternative release %d also has no durations", resultID)
				continue
			}

			logToFile("CrossReferenceTimestamps: Alternative release %d has durations, matching tracks", resultID)

			matchedTracks := matchTracksByName(currentTracks, altTracks)

			matchedWithDurations := 0
			for j, track := range matchedTracks {
				dur, _ := track["duration"].(int)
				trackTitle, _ := track["title"].(string)
				logToFile("CrossReferenceTimestamps: Matched track %d: %q duration=%d", j, trackTitle, dur)
				if dur > 0 {
					matchedWithDurations++
				}
			}

			logToFile("CrossReferenceTimestamps: Matched %d/%d tracks with durations from release %d",
				matchedWithDurations, len(matchedTracks), resultID)

			if matchedWithDurations > 0 {
				logToFile("CrossReferenceTimestamps: SUCCESS - matched %d tracks from release %d", len(matchedTracks), resultID)
				return matchedTracks, nil
			}
		}
	}

	logToFile("CrossReferenceTimestamps: No suitable alternative release found for %s - %s after searching %d results", artist, title, len(allResults))
	return currentTracks, nil
}

func normalizeStringForCompare(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "the ", "")
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "/", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func matchTracksByName(currentTracks, altTracks []map[string]interface{}) []map[string]interface{} {
	matched := make([]map[string]interface{}, len(currentTracks))

	for i, currentTrack := range currentTracks {
		currentTitle, _ := currentTrack["title"].(string)
		currentTitle = strings.TrimSpace(strings.ToLower(currentTitle))

		bestMatch := -1
		bestScore := 0.0

		for j, altTrack := range altTracks {
			altTitle, _ := altTrack["title"].(string)
			altTitle = strings.TrimSpace(strings.ToLower(altTitle))

			if currentTitle == altTitle {
				bestMatch = j
				bestScore = 1.0
				break
			}

			score := stringSimilarity(currentTitle, altTitle)
			if score > bestScore && score >= 0.7 {
				bestScore = score
				bestMatch = j
			}
		}

		if bestMatch >= 0 {
			altTrack := altTracks[bestMatch]
			duration, _ := altTrack["duration"].(int)
			if duration > 0 {
				matched[i] = map[string]interface{}{
					"track_number": currentTrack["track_number"],
					"disc_number":  currentTrack["disc_number"],
					"position":     currentTrack["position"],
					"title":        currentTrack["title"],
					"duration":     duration,
				}
				continue
			}
		}

		matched[i] = currentTrack
	}

	return matched
}

func removeZeroWidthChars(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 0x200B && r <= 0x200F {
			continue
		}
		if r == 0xFEFF {
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

func stringSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	distance := levenshteinDistance(a, b)
	maxLen := max(len(a), len(b))

	return 1.0 - (float64(distance) / float64(maxLen))
}

func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}

	return matrix[len(a)][len(b)]
}

func percentEncode(s string) string {
	return url.QueryEscape(s)
}

func percentEncodeValue(s string) string {
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "!", "%21")
	s = strings.ReplaceAll(s, "#", "%23")
	s = strings.ReplaceAll(s, "$", "%24")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "'", "%27")
	s = strings.ReplaceAll(s, "(", "%28")
	s = strings.ReplaceAll(s, ")", "%29")
	s = strings.ReplaceAll(s, "*", "%2A")
	s = strings.ReplaceAll(s, "+", "%2B")
	s = strings.ReplaceAll(s, ",", "%2C")
	s = strings.ReplaceAll(s, "/", "%2F")
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ";", "%3B")
	s = strings.ReplaceAll(s, "=", "%3D")
	s = strings.ReplaceAll(s, "?", "%3F")
	s = strings.ReplaceAll(s, "@", "%40")
	s = strings.ReplaceAll(s, "[", "%5B")
	s = strings.ReplaceAll(s, "]", "%5D")
	return s
}
