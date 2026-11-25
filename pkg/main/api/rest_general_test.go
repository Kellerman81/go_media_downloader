package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
)

// TestQueueCancelReplyShape cancels a running job through the route and
// checks the reply is flat: the queue page reads data.success, and a nested
// {"data":{...}} reply made a successful cancel show as failed.
func TestQueueCancelReplyShape(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	syncops.InitSyncOps()
	worker.RegisterWorkerSyncMaps()
	worker.InitWorkerPools(1, 1, 1, 1, 1, 1)
	t.Cleanup(worker.CloseWorkerPools)

	started := make(chan struct{})
	canceled := make(chan struct{})

	err := worker.Dispatch("test_cancel_reply", func(_ uint32, ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(canceled)

		return nil
	}, worker.QueueData)
	if err != nil {
		t.Fatal(err)
	}

	<-started

	var id uint32
	for qid := range worker.GetQueues() {
		id = qid
	}

	r := gin.New()
	r.DELETE("/api/queue/cancel/:id", apiQueueCancel)

	cancel := func(qid string) (int, map[string]any) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/queue/cancel/"+qid, nil))

		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("reply %q: %v", w.Body.String(), err)
		}

		return w.Code, body
	}

	code, body := cancel(strconv.FormatUint(uint64(id), 10))
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("cancel = %d %v, want 200 with top-level success:true", code, body)
	}

	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("job context was not canceled")
	}

	code, body = cancel("12345")
	if code != http.StatusNotFound || body["success"] != false || body["error"] == nil {
		t.Errorf("unknown id = %d %v, want 404 with success:false and an error", code, body)
	}
}
