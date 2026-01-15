package discogs

import (
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

const (
	APIURL          = "https://api.discogs.com"
	AuthURL         = "https://www.discogs.com/oauth"
	RateLimitWindow = 60 * time.Second
	AuthRequests    = 60
	AnonRequests    = 25
)

type RateLimiter struct {
	sync.RWMutex
	windowStart time.Time
	authCount   int
	anonCount   int
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{windowStart: time.Now()}
}

func (rl *RateLimiter) Wait(isAuth bool) {
	rl.Lock()
	defer rl.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.windowStart)

	fmt.Printf("[RateLimiter] Wait called: isAuth=%v, windowElapsed=%v, authCount=%d, anonCount=%d\n",
		isAuth, elapsed, rl.authCount, rl.anonCount)

	if elapsed >= RateLimitWindow {
		rl.windowStart = now
		rl.authCount = 0
		rl.anonCount = 0
		fmt.Printf("[RateLimiter] Window reset\n")
	}

	maxCount := AuthRequests
	if !isAuth {
		maxCount = AnonRequests
	}

	currentCount := &rl.authCount
	if !isAuth {
		currentCount = &rl.anonCount
	}

	for *currentCount >= maxCount {
		sleepTime := rl.windowStart.Add(RateLimitWindow).Sub(now)
		fmt.Printf("[RateLimiter] Rate limit reached, sleeping for %v\n", sleepTime)
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
		rl.windowStart = time.Now()
		rl.authCount = 0
		rl.anonCount = 0
		*currentCount = 0
	}

	*currentCount++
	fmt.Printf("[RateLimiter] Incremented count to %d\n", *currentCount)
}

func (rl *RateLimiter) UpdateFromHeaders(resp *http.Response) {
	rl.Lock()
	defer rl.Unlock()

	if rlAuth := resp.Header.Get("X-Discogs-Ratelimit-Auth"); rlAuth != "" {
		if limit, err := strconv.Atoi(rlAuth); err == nil {
			remaining := resp.Header.Get("X-Discogs-Ratelimit-Auth-Remaining")
			if rem, err := strconv.Atoi(remaining); err == nil && rem == 0 {
				rl.authCount = limit
			}
		}
	}

	if rlAnon := resp.Header.Get("X-Discogs-Ratelimit"); rlAnon != "" {
		if limit, err := strconv.Atoi(rlAnon); err == nil {
			remaining := resp.Header.Get("X-Discogs-Ratelimit-Remaining")
			if rem, err := strconv.Atoi(remaining); err == nil && rem == 0 {
				rl.anonCount = limit
			}
		}
	}
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
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		RateLimiter: NewRateLimiter(),
		OAuth:       loadOAuthConfig(),
	}
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
	isAuth := c.IsAuthenticated() && c.APIKey == ""
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Discogs API error: %d - %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func (c *Client) makeOAuthRequest(method, requestURL string, body url.Values) (*http.Response, error) {
	c.RateLimiter.Wait(true)

	if body == nil {
		body = url.Values{}
	}

	body.Set("oauth_token", c.OAuth.AccessToken)

	fmt.Printf("DEBUG [makeOAuthRequest] === OAuth Request Details ===\n")
	fmt.Printf("DEBUG [makeOAuthRequest] URL: %s\n", requestURL)
	fmt.Printf("DEBUG [makeOAuthRequest] Method: %s\n", method)
	fmt.Printf("DEBUG [makeOAuthRequest] Body: %s\n", body.Encode())
	fmt.Printf("DEBUG [makeOAuthRequest] AccessToken: %s\n", maskValue(c.OAuth.AccessToken))
	fmt.Printf("DEBUG [makeOAuthRequest] ConsumerKey: %s\n", maskValue(c.OAuth.ConsumerKey))

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	fmt.Printf("DEBUG [makeOAuthRequest] Timestamp: %s\n", timestamp)
	fmt.Printf("DEBUG [makeOAuthRequest] Nonce: %s\n", nonce)

	oauthParams := map[string]string{
		"oauth_consumer_key":     c.OAuth.ConsumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            c.OAuth.AccessToken,
		"oauth_version":          "1.0",
	}

	fmt.Printf("DEBUG [makeOAuthRequest] OAuth Params:\n")
	for k, v := range oauthParams {
		if k == "oauth_consumer_key" || k == "oauth_token" {
			fmt.Printf("DEBUG [makeOAuthRequest]   %s: %s\n", k, maskValue(v))
		} else {
			fmt.Printf("DEBUG [makeOAuthRequest]   %s: %s\n", k, v)
		}
	}

	params := make(url.Values)
	for k, v := range oauthParams {
		params.Set(k, v)
	}

	for k, v := range body {
		params[k] = v
	}

	var paramKeys []string
	for k := range params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)

	var paramPairs []string
	for _, k := range paramKeys {
		paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(params.Get(k))))
	}

	baseString := fmt.Sprintf("%s&%s&%s", method, url.QueryEscape(requestURL), url.QueryEscape(strings.Join(paramPairs, "&")))

	fmt.Printf("DEBUG [makeOAuthRequest] === Signature Base String ===\n")
	fmt.Printf("DEBUG [makeOAuthRequest] baseString: %s\n", baseString)

	signature := generateHmacSignature(baseString, c.OAuth.ConsumerSecret, c.OAuth.AccessSecret)

	fmt.Printf("DEBUG [makeOAuthRequest] === Generated Signature ===\n")
	fmt.Printf("DEBUG [makeOAuthRequest] signature: %s\n", signature)
	fmt.Printf("DEBUG [makeOAuthRequest] ConsumerSecret: %s\n", maskValue(c.OAuth.ConsumerSecret))
	fmt.Printf("DEBUG [makeOAuthRequest] TokenSecret: %s\n", maskValue(c.OAuth.AccessSecret))

	authHeader := fmt.Sprintf(`OAuth oauth_consumer_key="%s", oauth_token="%s", oauth_signature_method="HMAC-SHA1", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_signature="%s"`,
		url.QueryEscape(c.OAuth.ConsumerKey),
		url.QueryEscape(c.OAuth.AccessToken),
		timestamp,
		nonce,
		url.QueryEscape(signature),
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
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", "\"Windows\"")

	fmt.Printf("DEBUG makeOAuthRequest: Authorization=%s\n", authHeader)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	c.RateLimiter.UpdateFromHeaders(resp)

	fmt.Printf("DEBUG makeOAuthRequest: response status=%d, headers=%v\n", resp.StatusCode, resp.Header)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Discogs API error: %d - %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func generateHmacSignature(baseString, consumerSecret, tokenSecret string) string {
	key := fmt.Sprintf("%s&%s", url.QueryEscape(consumerSecret), url.QueryEscape(tokenSecret))

	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

func (c *Client) GetRequestToken() (token, secret, authURL string, err error) {
	consumerKey := os.Getenv("DISCOGS_CONSUMER_KEY")
	consumerSecret := os.Getenv("DISCOGS_CONSUMER_SECRET")
	callbackURL := os.Getenv("DISCOGS_CALLBACK_URL")

	fmt.Printf("=== [GetRequestToken] OAuth 1.0a Request Token Flow ===\n")
	fmt.Printf("[GetRequestToken] ConsumerKey: %s\n", maskValue(consumerKey))
	fmt.Printf("[GetRequestToken] ConsumerSecret: %s\n", maskValue(consumerSecret))
	fmt.Printf("[GetRequestToken] CallbackURL: %s\n", callbackURL)

	if consumerKey == "" || consumerSecret == "" {
		return "", "", "", fmt.Errorf("DISCOGS_CONSUMER_KEY or DISCOGS_CONSUMER_SECRET not set")
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	fmt.Printf("[GetRequestToken] Timestamp: %s\n", timestamp)
	fmt.Printf("[GetRequestToken] Nonce: %s\n", nonce)

	oauthParams := fmt.Sprintf("oauth_callback=%s&oauth_consumer_key=%s&oauth_nonce=%s&oauth_signature_method=HMAC-SHA1&oauth_timestamp=%s&oauth_version=1.0",
		url.QueryEscape(callbackURL),
		url.QueryEscape(consumerKey),
		nonce,
		timestamp)

	baseString := fmt.Sprintf("%s&%s&%s", "POST", url.QueryEscape(AuthURL+"/request_token"), url.QueryEscape(oauthParams))

	fmt.Printf("[GetRequestToken] === Signature Base String ===\n")
	fmt.Printf("[GetRequestToken] baseString: %s\n", baseString)

	signature := generateHmacSignature(baseString, consumerSecret, "")

	fmt.Printf("[GetRequestToken] === Generated Signature ===\n")
	fmt.Printf("[GetRequestToken] signature: %s\n", signature)

	authHeader := fmt.Sprintf(`OAuth oauth_consumer_key="%s", oauth_signature="%s", oauth_signature_method="HMAC-SHA1", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_callback="%s"`,
		url.QueryEscape(consumerKey),
		url.QueryEscape(signature),
		timestamp,
		nonce,
		url.QueryEscape(callbackURL))

	fmt.Printf("[GetRequestToken] Authorization Header: %s\n", authHeader)

	fmt.Printf("[GetRequestToken] Making request to Discogs OAuth endpoint...\n")

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

	fmt.Printf("[GetRequestToken] Response status=%d\n", resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("[GetRequestToken] Response body length=%d\n", len(body))

	if resp.StatusCode != 200 {
		return "", "", "", fmt.Errorf("Discogs OAuth request failed with status %d. This is likely due to Cloudflare protection blocking automated requests. Please complete OAuth manually through a web browser.", resp.StatusCode)
	}

	values, _ := url.ParseQuery(string(body))

	token = values.Get("oauth_token")
	secret = values.Get("oauth_token_secret")
	authURL = AuthURL + "/authorize?oauth_token=" + token

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

	fmt.Printf("=== [GetAccessToken] OAuth 1.0a Access Token Flow ===\n")
	fmt.Printf("[GetAccessToken] Request Token: %s\n", maskValue(token))
	fmt.Printf("[GetAccessToken] Request Token Secret: %s\n", maskValue(secret))
	fmt.Printf("[GetAccessToken] Verifier: %s\n", verifier)
	fmt.Printf("[GetAccessToken] ConsumerKey: %s\n", maskValue(c.OAuth.ConsumerKey))
	fmt.Printf("[GetAccessToken] ConsumerSecret: %s\n", maskValue(c.OAuth.ConsumerSecret))

	if c.OAuth.ConsumerKey == "" || c.OAuth.ConsumerSecret == "" {
		return "", "", "", fmt.Errorf("DISCOGS_CONSUMER_KEY or DISCOGS_CONSUMER_SECRET not set")
	}

	data := url.Values{}
	data.Set("oauth_token", token)
	data.Set("oauth_verifier", verifier)

	fmt.Printf("[GetAccessToken] Request Body: %s\n", data.Encode())
	fmt.Printf("[GetAccessToken] Calling: %s/oauth/access_token\n", AuthURL)

	resp, err := c.makeOAuthRequest("POST", AuthURL+"/access_token", data)
	if err != nil {
		fmt.Printf("[GetAccessToken] ERROR: %v\n", err)
		return "", "", "", err
	}
	defer resp.Body.Close()

	fmt.Printf("[GetAccessToken] Response Status: %d\n", resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("[GetAccessToken] Response Body: %s\n", string(body))
	values, _ := url.ParseQuery(string(body))

	accessToken = values.Get("oauth_token")
	accessSecret = values.Get("oauth_token_secret")
	username = values.Get("username")

	fmt.Printf("[GetAccessToken] === Access Token Received ===\n")
	fmt.Printf("[GetAccessToken] AccessToken: %s\n", maskValue(accessToken))
	fmt.Printf("[GetAccessToken] AccessSecret: %s\n", maskValue(accessSecret))
	fmt.Printf("[GetAccessToken] Username: %s\n", username)

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

func (c *Client) GetUserCollection(page, perPage int) ([]map[string]interface{}, error) {
	requestURL := fmt.Sprintf("%s/users/%s/collection/folders/0/releases?page=%d&per_page=%d",
		APIURL, c.OAuth.AccessToken, page, perPage)

	resp, err := c.makeOAuthRequest("GET", requestURL, nil)
	if err != nil {
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
		})
	}

	return releases, nil
}

func (c *Client) SearchAlbums(query string, page int) ([]map[string]interface{}, int, error) {
	searchURL := fmt.Sprintf("%s/database/search?q=%s&type=release&page=%d&per_page=10&sort=year&sort_order=desc",
		APIURL, url.QueryEscape(query), page)

	resp, err := c.makeRequest("GET", searchURL, nil)
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
	resp, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	album, err := parseAlbumResponse(resp)
	if err != nil {
		return nil, err
	}

	return album, nil
}

func parseAlbumResponse(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()

	var discogsAlbum struct {
		ID      int      `json:"id"`
		Title   string   `json:"title"`
		Year    int      `json:"year"`
		Country string   `json:"country"`
		Genres  []string `json:"genres"`
		Styles  []string `json:"styles"`
		Images  []struct {
			URI  string `json:"uri"`
			Type string `json:"type"`
		} `json:"images"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		Tracklist []struct {
			Title    string `json:"title"`
			Duration string `json:"duration"`
			Position string `json:"position"`
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

	album := map[string]interface{}{
		"discogs_id":  discogsAlbum.ID,
		"title":       discogsAlbum.Title,
		"artist":      artistName,
		"year":        discogsAlbum.Year,
		"country":     discogsAlbum.Country,
		"genre":       genre,
		"styles":      discogsAlbum.Styles,
		"cover_image": coverImage,
		"tracklist":   parseTracklist(discogsAlbum.Tracklist),
	}

	return album, nil
}

func parseTracklist(tracklist []struct {
	Title    string `json:"title"`
	Duration string `json:"duration"`
	Position string `json:"position"`
}) []map[string]interface{} {
	tracks := make([]map[string]interface{}, 0)
	for i, track := range tracklist {
		tracks = append(tracks, map[string]interface{}{
			"track_number": i + 1,
			"position":     track.Position,
			"title":        track.Title,
			"duration":     track.Duration,
		})
	}
	return tracks
}

func maskValue(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

func (c *Client) GetTracksForAlbum(id int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/releases/%d", APIURL, id)
	resp, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	album, err := parseAlbumResponse(resp)
	if err != nil {
		return nil, err
	}

	return album["tracklist"].([]map[string]interface{}), nil
}
