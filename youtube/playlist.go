package youtube

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/tidwall/gjson"
)

type PlayListData struct {
	Kind          string `json:"kind"`
	Etag          string `json:"etag"`
	NextPageToken string `json:"nextPageToken"`
	Items         []struct {
		Kind    string `json:"kind"`
		Etag    string `json:"etag"`
		ID      string `json:"id"`
		Snippet struct {
			PublishedAt time.Time `json:"publishedAt"`
			ChannelID   string    `json:"channelId"`
			Title       string    `json:"title"`
			Description string    `json:"description"`
			Thumbnails  struct {
				Default struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"default"`
				Medium struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"medium"`
				High struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"high"`
				Standard struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"standard"`
				Maxres struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"maxres"`
			} `json:"thumbnails"`
			ChannelTitle string `json:"channelTitle"`
			PlaylistID   string `json:"playlistId"`
			Position     int    `json:"position"`
			ResourceID   struct {
				Kind    string `json:"kind"`
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
			VideoOwnerChannelTitle string `json:"videoOwnerChannelTitle"`
			VideoOwnerChannelID    string `json:"videoOwnerChannelId"`
		} `json:"snippet"`
		ContentDetails struct {
			VideoID          string    `json:"videoId"`
			VideoPublishedAt time.Time `json:"videoPublishedAt"`
		} `json:"contentDetails"`
	} `json:"items"`
	PageInfo struct {
		TotalResults   int `json:"totalResults"`
		ResultsPerPage int `json:"resultsPerPage"`
	} `json:"pageInfo"`
}

func ParsePlaylist(_url string) ([]string, error) {
	parsedUrl, err := url.ParseRequestURI(_url)
	if err != nil {
		return nil, err
	}

	_url = parsedUrl.Query().Get("list")
	if _url == "" {
		return nil, errors.New("invalid_playlist_url")
	}
	var ytApikey = os.Getenv("YT_APIKEY")
	req, err := http.Get("https://youtube.googleapis.com/youtube/v3/playlistItems?part=snippet%2CcontentDetails&playlistId=" + _url + "&key=" + ytApikey)
	if err != nil {
		return nil, err
	}

	res, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}

	isError := gjson.Get(string(res), "error.code")
	if isError.Int() != 0 {
		return nil, errors.New("playlist_not_found")
	}

	lists := &PlayListData{}
	err = json.Unmarshal(res, lists)
	if err != nil {
		return nil, err
	}

	response := []string{}
	for _, v := range lists.Items {
		if v.Snippet.ResourceID.Kind == "youtube#video" {
			response = append(response, "https://youtu.be/"+v.Snippet.ResourceID.VideoID)
		}
	}
	return response, nil
}
