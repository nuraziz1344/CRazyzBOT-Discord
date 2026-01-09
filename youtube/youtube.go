package youtube

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/tidwall/gjson"
)

type PlayResponse struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Bitrate int    `json:"bitrate"`
}

func Search(keyword string) (string, error) {
	var ytApikey = os.Getenv("YT_APIKEY")
	_url := fmt.Sprintf("https://youtube.googleapis.com/youtube/v3/search?q=%s&type=video&key=%s", url.QueryEscape(keyword), ytApikey)
	req, err := http.Get(_url)
	if err != nil {
		return "", err
	}

	defer req.Body.Close()
	res, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}

	data := string(res)

	vId := gjson.Get(data, "items.1.id.videoId")

	return "https://youtu.be/" + vId.Str, nil
}

func Play(_url string) (*PlayResponse, error) {
	if _url[:4] != "http" {
		ytUrl, err := Search(_url)
		if err != nil {
			return &PlayResponse{}, err
		}
		_url = ytUrl
	}

	title, url, err := YtMp3(_url)
	if err != nil {
		return &PlayResponse{}, err
	} else if url == "" {
		return &PlayResponse{}, errors.New("ytmp3_failed")
	}

	response := &PlayResponse{Title: title, URL: url, Bitrate: 128}
	return response, nil
}
