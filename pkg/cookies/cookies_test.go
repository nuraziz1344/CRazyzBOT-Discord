package cookies

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndApplyNetscapeCookies(t *testing.T) {
	tmpDir := t.TempDir()
	cookieFile := filepath.Join(tmpDir, "cookies.txt")

	content := `# Netscape HTTP Cookie File
.youtube.com	TRUE	/	TRUE	0	SOCS	CAI
.spotify.com	TRUE	/	TRUE	0	sp_dc	test_spotify_token
`
	if err := os.WriteFile(cookieFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp cookie file: %v", err)
	}

	reqYT, _ := http.NewRequest(http.MethodGet, "https://music.youtube.com/youtubei/v1/search", nil)
	Apply(reqYT, cookieFile)

	c, err := reqYT.Cookie("SOCS")
	if err != nil || c.Value != "CAI" {
		t.Fatalf("expected SOCS cookie on music.youtube.com, got %v (err=%v)", c, err)
	}
	if _, err := reqYT.Cookie("sp_dc"); err == nil {
		t.Fatalf("did not expect spotify cookie on youtube request")
	}

	reqSP, _ := http.NewRequest(http.MethodGet, "https://open.spotify.com/embed/track/123", nil)
	Apply(reqSP, cookieFile)

	spCookie, err := reqSP.Cookie("sp_dc")
	if err != nil || spCookie.Value != "test_spotify_token" {
		t.Fatalf("expected sp_dc cookie on open.spotify.com, got %v (err=%v)", spCookie, err)
	}
}
