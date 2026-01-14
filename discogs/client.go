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
	if now.Sub(rl.windowStart) >= RateLimitWindow {
		rl.windowStart = now
		rl.authCount = 0
		rl.anonCount = 0
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
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
		rl.windowStart = time.Now()
		rl.authCount = 0
		rl.anonCount = 0
		*currentCount = 0
	}

	*currentCount++
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

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	baseString := fmt.Sprintf("%s&%s&%s", method, url.QueryEscape(requestURL), url.QueryEscape(
		fmt.Sprintf("oauth_consumer_key=%s&oauth_nonce=%s&oauth_signature_method=HMAC-SHA1&oauth_timestamp=%s&oauth_token=%s&oauth_version=1.0",
			c.OAuth.ConsumerKey, nonce, timestamp, c.OAuth.AccessToken)))

	authHeader := fmt.Sprintf(`OAuth oauth_consumer_key="%s", oauth_token="%s", oauth_signature_method="HMAC-SHA1", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_signature="%s"`,
		c.OAuth.ConsumerKey,
		c.OAuth.AccessToken,
		timestamp,
		nonce,
		url.QueryEscape(baseString),
	)

	req, err := http.NewRequest(method, requestURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Vinylfo/1.0 (https://github.com/xphox2/Vinylfo)")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	baseString := fmt.Sprintf("%s&%s&%s", "POST", url.QueryEscape(AuthURL+"/request_token"), url.QueryEscape(
		fmt.Sprintf("oauth_callback=%s&oauth_consumer_key=%s&oauth_nonce=%s&oauth_signature_method=HMAC-SHA1&oauth_timestamp=%s&oauth_version=1.0",
			url.QueryEscape(os.Getenv("DISCOGS_CALLBACK_URL")), url.QueryEscape(consumerKey), nonce, timestamp)))

	signature := generateHmacSignature(baseString, consumerSecret, "")

	authHeader := fmt.Sprintf(`OAuth oauth_consumer_key="%s", oauth_signature="%s", oauth_signature_method="HMAC-SHA1", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_callback="%s"`,
		url.QueryEscape(consumerKey),
		url.QueryEscape(signature),
		timestamp,
		nonce,
		url.QueryEscape(os.Getenv("DISCOGS_CALLBACK_URL")))

	req, err := http.NewRequest("POST", AuthURL+"/request_token", nil)
	if err != nil {
		return "", "", "", err
	}

	req.Header.Set("User-Agent", "Vinylfo/1.0 (https://github.com/xphox2/Vinylfo)")
	req.Header.Set("Authorization", authHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	values, _ := url.ParseQuery(string(body))

	token = values.Get("oauth_token")
	secret = values.Get("oauth_token_secret")
	authURL = AuthURL + "/authorize?oauth_token=" + token

	return token, secret, authURL, nil
}

func (c *Client) GetAccessToken(token, secret, verifier string) (accessToken, accessSecret, username string, err error) {
	c.OAuth.ConsumerKey = os.Getenv("DISCOGS_CONSUMER_KEY")
	c.OAuth.ConsumerSecret = os.Getenv("DISCOGS_CONSUMER_SECRET")
	c.OAuth.AccessToken = token
	c.OAuth.AccessSecret = secret

	data := url.Values{}
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

func (c *Client) SearchAlbums(query string, page int) ([]map[string]interface{}, error) {
	searchURL := fmt.Sprintf("%s/database/search?q=%s&type=release&page=%d&per_page=25",
		APIURL, url.QueryEscape(query), page)

	resp, err := c.makeRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResponse struct {
		Results []struct {
			ID      int    `json:"id"`
			Title   string `json:"title"`
			Year    int    `json:"year"`
			Country string `json:"country"`
			Format  string `json:"format"`
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
		return nil, err
	}

	albums := make([]map[string]interface{}, 0)
	for _, result := range searchResponse.Results {
		artistName := ""
		if len(result.Artists) > 0 {
			artistName = result.Artists[0].Name
		}

		albums = append(albums, map[string]interface{}{
			"discogs_id":  result.ID,
			"title":       result.Title,
			"artist":      artistName,
			"year":        result.Year,
			"country":     result.Country,
			"format":      result.Format,
			"cover_image": result.CoverImage,
		})
	}

	return albums, nil
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
