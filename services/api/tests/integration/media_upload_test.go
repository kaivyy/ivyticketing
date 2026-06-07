//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"
)

func TestMediaUpload_LocalDirectFlow(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Media Org")

	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events",
		map[string]any{"name": "Media Event", "eventType": "marathon"}, token)
	var ev struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&ev)
	resp.Body.Close()

	// 1. Request ticket for banner.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/media/banner",
		map[string]string{"contentType": "image/png", "fileName": "b.png"}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ticket = %d, want 200", resp.StatusCode)
	}
	var ticket struct {
		Mode      string `json:"mode"`
		ObjectKey string `json:"objectKey"`
		UploadURL string `json:"uploadUrl"`
	}
	json.NewDecoder(resp.Body).Decode(&ticket)
	resp.Body.Close()
	if ticket.Mode != "direct" {
		t.Fatalf("mode = %q, want direct (local)", ticket.Mode)
	}

	// 2. Multipart upload to the local sink.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// CreateFormFile sets Content-Type to application/octet-stream; create the
	// part manually so we can declare image/png as required by the handler.
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="b.png"`)
	partHeader.Set("Content-Type", "image/png")
	fw, _ := mw.CreatePart(partHeader)
	fw.Write([]byte("\x89PNG\r\n\x1a\nfakeimage"))
	mw.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+ticket.UploadURL, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("upload = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Confirm.
	body, _ := json.Marshal(map[string]string{"objectKey": ticket.ObjectKey})
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/media/banner/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm = %d, want 200", resp.StatusCode)
	}
	var confirmed struct{ BannerURL string `json:"bannerUrl"` }
	json.NewDecoder(resp.Body).Decode(&confirmed)
	resp.Body.Close()
	if confirmed.BannerURL == "" {
		t.Fatal("bannerUrl should be set after confirm")
	}

	// 4. The file is served at /media/{key}.
	resp, _ = client.Get(srv.URL + "/media/" + ticket.ObjectKey)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("serve media = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. Confirm rejects a tampered key (different event prefix).
	body, _ = json.Marshal(map[string]string{"objectKey": "org/" + orgID + "/event/00000000-0000-0000-0000-000000000000/banner/x.png"})
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/media/banner/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("tampered confirm = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}
