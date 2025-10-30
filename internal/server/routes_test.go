package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"log/slog"

	"github.com/arimatakao/simple-events-handler/internal/database"
	"github.com/gin-gonic/gin"
)

// mockDB implements the database.Service interface minimally for testing.
type mockDB struct {
	insertCalled bool
	lastUserID   int64
	lastAction   string
	lastMeta     map[string]string
	insertID     int64
	insertErr    error
	// get events
	getCalled  bool
	getUserID  *int64
	getStart   *time.Time
	getEnd     *time.Time
	getResults []database.Event
	getErr     error
}

func (m *mockDB) Health() map[string]string { return map[string]string{"status": "ok"} }
func (m *mockDB) Close() error              { return nil }
func (m *mockDB) InsertEvent(ctx context.Context, userID int64, action string, metadata map[string]string) (int64, error) {
	m.insertCalled = true
	m.lastUserID = userID
	m.lastAction = action
	m.lastMeta = metadata
	return m.insertID, m.insertErr
}
func (m *mockDB) GetEvents(ctx context.Context, userID *int64, start *time.Time, end *time.Time) ([]database.Event, error) {
	m.getCalled = true
	m.getUserID = userID
	m.getStart = start
	m.getEnd = end
	return m.getResults, m.getErr
}
func (m *mockDB) AggregateEvents(seconds int) error { return nil }

// TestAddEventHandler_Success ensures that a valid POST /events calls InsertEvent and returns 201.
func TestAddEventHandler(t *testing.T) {
	// silent logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name           string
		mockSetup      func() *mockDB
		requestBody    []byte
		expectedStatus int
		expectDBCalled bool
	}{
		{
			name: "success",
			mockSetup: func() *mockDB {
				return &mockDB{insertID: 42}
			},
			requestBody: func() []byte {
				b, _ := json.Marshal(AddEventRequest{UserID: 1, Action: "click", Metadata: map[string]string{"page": "home"}})
				return b
			}(),
			expectedStatus: http.StatusCreated,
			expectDBCalled: true,
		},
		{
			name: "invalid json",
			mockSetup: func() *mockDB {
				return &mockDB{}
			},
			requestBody:    []byte("{bad json}"),
			expectedStatus: http.StatusBadRequest,
			expectDBCalled: false,
		},
		{
			name: "validation: missing action",
			mockSetup: func() *mockDB {
				return &mockDB{}
			},
			requestBody: func() []byte {
				b, _ := json.Marshal(AddEventRequest{UserID: 1, Action: "", Metadata: nil})
				return b
			}(),
			expectedStatus: http.StatusBadRequest,
			expectDBCalled: false,
		},
		{
			name: "validation: non-positive user id",
			mockSetup: func() *mockDB {
				return &mockDB{}
			},
			requestBody: func() []byte {
				b, _ := json.Marshal(AddEventRequest{UserID: 0, Action: "click", Metadata: nil})
				return b
			}(),
			expectedStatus: http.StatusBadRequest,
			expectDBCalled: false,
		},
		{
			name: "db insert error",
			mockSetup: func() *mockDB {
				return &mockDB{insertErr: fmt.Errorf("boom")}
			},
			requestBody: func() []byte {
				b, _ := json.Marshal(AddEventRequest{UserID: 1, Action: "click", Metadata: nil})
				return b
			}(),
			expectedStatus: http.StatusInternalServerError,
			expectDBCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.mockSetup()

			s := &Server{
				l:  logger,
				db: mock,
			}

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/events", s.AddEventHandler)

			req, err := http.NewRequest("POST", "/events", bytes.NewReader(tt.requestBody))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Fatalf("%s: expected status %d got %d, body: %s", tt.name, tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.expectDBCalled && !mock.insertCalled {
				t.Fatalf("%s: expected InsertEvent to be called", tt.name)
			}
			if !tt.expectDBCalled && mock.insertCalled {
				t.Fatalf("%s: expected InsertEvent not to be called", tt.name)
			}

			// additional checks for success case
			if tt.name == "success" {
				if mock.lastUserID != 1 {
					t.Fatalf("expected user id 1 got %d", mock.lastUserID)
				}
				if mock.lastAction != "click" {
					t.Fatalf("expected action 'click' got %q", mock.lastAction)
				}
				if v := mock.lastMeta["page"]; v != "home" {
					t.Fatalf("expected metadata page 'home' got %q", v)
				}
			}
		})
	}
}

// TestGetEventsHandler covers GET /events behavior with various query parameters.
func TestGetEventsHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)

	tests := []struct {
		name           string
		mockSetup      func() *mockDB
		query          string
		expectedStatus int
		expectDBCalled bool
		expectResults  []database.Event
	}{
		{
			name: "success with user",
			mockSetup: func() *mockDB {
				return &mockDB{getResults: []database.Event{{ID: 1, UserID: 1, Action: "click", MetadataPage: nil, CreatedAt: now}}}
			},
			query:          "?user_id=1&from=" + url.QueryEscape(earlier.Format(time.RFC3339)) + "&to=" + url.QueryEscape(now.Format(time.RFC3339)),
			expectedStatus: http.StatusOK,
			expectDBCalled: true,
			expectResults:  []database.Event{{ID: 1, UserID: 1, Action: "click", MetadataPage: nil, CreatedAt: now}},
		},
		{
			name: "invalid user_id",
			mockSetup: func() *mockDB {
				return &mockDB{}
			},
			query:          "?user_id=bad&from=2020-01-01T00:00:00Z&to=2020-01-02T00:00:00Z",
			expectedStatus: http.StatusBadRequest,
			expectDBCalled: false,
		},
		{
			name: "missing from",
			mockSetup: func() *mockDB {
				return &mockDB{}
			},
			query:          "?user_id=1&to=2020-01-02T00:00:00Z",
			expectedStatus: http.StatusBadRequest,
			expectDBCalled: false,
		},
		{
			name: "invalid time format",
			mockSetup: func() *mockDB {
				return &mockDB{}
			},
			query:          "?from=not-a-time&to=also-not-a-time",
			expectedStatus: http.StatusBadRequest,
			expectDBCalled: false,
		},
		{
			name: "from after to",
			mockSetup: func() *mockDB {
				return &mockDB{}
			},
			query:          "?from=2020-01-02T00:00:00Z&to=2020-01-01T00:00:00Z",
			expectedStatus: http.StatusBadRequest,
			expectDBCalled: false,
		},
		{
			name: "db error",
			mockSetup: func() *mockDB {
				return &mockDB{getErr: fmt.Errorf("boom")}
			},
			query:          "?from=2020-01-01T00:00:00Z&to=2020-01-02T00:00:00Z",
			expectedStatus: http.StatusInternalServerError,
			expectDBCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.mockSetup()

			s := &Server{
				l:  logger,
				db: mock,
			}

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/events", s.GetEventsHandler)

			req, err := http.NewRequest("GET", "/events"+tt.query, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Fatalf("%s: expected status %d got %d, body: %s", tt.name, tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.expectDBCalled && !mock.getCalled {
				t.Fatalf("%s: expected GetEvents to be called", tt.name)
			}
			if !tt.expectDBCalled && mock.getCalled {
				t.Fatalf("%s: expected GetEvents not to be called", tt.name)
			}

			if tt.expectedStatus == http.StatusOK {
				// decode response body
				var got []database.Event
				if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(got) != len(tt.expectResults) {
					t.Fatalf("expected %d results got %d", len(tt.expectResults), len(got))
				}
				// basic field check
				for i := range got {
					if got[i].ID != tt.expectResults[i].ID || got[i].UserID != tt.expectResults[i].UserID || got[i].Action != tt.expectResults[i].Action {
						t.Fatalf("result mismatch: expected %+v got %+v", tt.expectResults[i], got[i])
					}
				}
			}
		})
	}
}
