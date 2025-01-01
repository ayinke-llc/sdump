package httpd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ayinke-llc/sdump"
	"github.com/ayinke-llc/sdump/config"
	"github.com/ayinke-llc/sdump/mocks"
	"github.com/r3labs/sse/v2"
	"github.com/sebdah/goldie/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func verifyMatch(t *testing.T, v interface{}) {
	g := goldie.New(t, goldie.WithFixtureDir("./testdata"))

	b := new(bytes.Buffer)

	var err error

	if d, ok := v.(*httptest.ResponseRecorder); ok {
		_, err = io.Copy(b, d.Body)
	} else {
		err = json.NewEncoder(b).Encode(v)
	}

	require.NoError(t, err)
	g.Assert(t, t.Name(), b.Bytes())
}

func TestURLHandler_Create(t *testing.T) {
	tt := []struct {
		name   string
		mockFn func(t *testing.T, urlRepo *mocks.MockURLRepository,
			userRepo *mocks.MockUserRepository)
		expectedStatusCode int
		requestBody        createURLRequest

		// sometimes data changes in the response, if this
		// field is set to true, we will skip matching golden files
		// technically it can be reworked to provide an implementation that never
		// changes during tests but I can always come back to taht
		hasDynamicData bool
	}{
		{
			name:               "ssh fingerprint not provided",
			expectedStatusCode: http.StatusBadRequest,
			mockFn: func(t *testing.T, _ *mocks.MockURLRepository, _ *mocks.MockUserRepository) {
			},
		},
		{
			name: "error occured while fetching user by ssh fingerprint",
			mockFn: func(t *testing.T, urlRepo *mocks.MockURLRepository,
				userRepo *mocks.MockUserRepository,
			) {
				t.Helper()
				userRepo.EXPECT().Find(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("could not fetch user"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			requestBody: createURLRequest{
				SSHFingerprint: "sufojfpffhhofjfpjfo",
			},
		},
		{
			name: "user not found and could not be created also",
			mockFn: func(t *testing.T, urlRepo *mocks.MockURLRepository,
				userRepo *mocks.MockUserRepository,
			) {
				t.Helper()
				userRepo.EXPECT().Find(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sdump.ErrUserNotFound)

				userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("could not create user"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			requestBody: createURLRequest{
				SSHFingerprint: "sufojfpffhhofjfpjfo",
			},
		},
		{
			name: "url could not be created",
			mockFn: func(t *testing.T, urlRepo *mocks.MockURLRepository,
				userRepo *mocks.MockUserRepository,
			) {
				t.Helper()
				userRepo.EXPECT().Find(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sdump.ErrUserNotFound)

				userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				urlRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("could not create url"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			requestBody: createURLRequest{
				SSHFingerprint:   "sufojfpffhhofjfpjfo",
				ForceNewEndpoint: true,
			},
		},
		{
			name:           "url was successfully created",
			hasDynamicData: true,
			mockFn: func(t *testing.T, urlRepo *mocks.MockURLRepository,
				userRepo *mocks.MockUserRepository,
			) {
				t.Helper()
				userRepo.EXPECT().Find(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sdump.ErrUserNotFound)

				userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				urlRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			requestBody: createURLRequest{
				SSHFingerprint:   "sufojfpffhhofjfpjfo",
				ForceNewEndpoint: true,
			},
		},
		{
			name: "user already exists, no need to create a new url. could not fetch existing url",
			mockFn: func(t *testing.T, urlRepo *mocks.MockURLRepository,
				userRepo *mocks.MockUserRepository,
			) {
				t.Helper()
				userRepo.EXPECT().Find(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&sdump.User{}, nil)

				urlRepo.EXPECT().Latest(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("could not fetch latest url"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			requestBody: createURLRequest{
				SSHFingerprint: "sufojfpffhhofjfpjfo",
			},
		},
		{
			name: "user already exists, no need to create a new url. existing url not found, could not create new one",
			mockFn: func(t *testing.T, urlRepo *mocks.MockURLRepository,
				userRepo *mocks.MockUserRepository,
			) {
				t.Helper()
				userRepo.EXPECT().Find(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&sdump.User{}, nil)

				urlRepo.EXPECT().Latest(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sdump.ErrURLEndpointNotFound)

				urlRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("could not create url"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			requestBody: createURLRequest{
				SSHFingerprint: "sufojfpffhhofjfpjfo",
			},
		},
		{
			name:           "user already exists, no need to create a new url. existing url found",
			hasDynamicData: true,
			mockFn: func(t *testing.T, urlRepo *mocks.MockURLRepository,
				userRepo *mocks.MockUserRepository,
			) {
				t.Helper()
				userRepo.EXPECT().Find(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&sdump.User{}, nil)

				urlRepo.EXPECT().Latest(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&sdump.URLEndpoint{}, nil)
			},
			expectedStatusCode: http.StatusOK,
			requestBody: createURLRequest{
				SSHFingerprint: "sufojfpffhhofjfpjfo",
			},
		},
	}

	for _, v := range tt {
		t.Run(v.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			b := new(bytes.Buffer)

			err := json.NewEncoder(b).Encode(v.requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/", b)

			logrus.SetOutput(io.Discard)

			logger := logrus.WithField("module", "test")

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			urlRepo := mocks.NewMockURLRepository(ctrl)
			userRepo := mocks.NewMockUserRepository(ctrl)

			v.mockFn(t, urlRepo, userRepo)

			u := &urlHandler{
				logger:    logger,
				cfg:       config.Config{},
				urlRepo:   urlRepo,
				userRepo:  userRepo,
				sseServer: sse.New(),
			}

			u.create(recorder, req)

			require.Equal(t, v.expectedStatusCode, recorder.Result().StatusCode)

			if !v.hasDynamicData {
				verifyMatch(t, recorder)
			}
		})
	}
}

func TestURLHandler_Ingest(t *testing.T) {
	tt := []struct {
		name               string
		mockFn             func(urlRepo *mocks.MockURLRepository, requestRepo *mocks.MockIngestRepository)
		expectedStatusCode int
		requestBody        io.Reader
		requestBodySize    int64
	}{
		{
			name: "url reference not found",
			mockFn: func(urlRepo *mocks.MockURLRepository, requestRepo *mocks.MockIngestRepository) {
				urlRepo.EXPECT().Get(gomock.Any(), gomock.Any()).
					Times(1).Return(&sdump.URLEndpoint{}, sdump.ErrURLEndpointNotFound)
			},
			expectedStatusCode: http.StatusNotFound,
			requestBody:        strings.NewReader(``),
			requestBodySize:    10,
		},
		{
			name: "error while fetching url",
			mockFn: func(urlRepo *mocks.MockURLRepository, requestRepo *mocks.MockIngestRepository) {
				urlRepo.EXPECT().Get(gomock.Any(), gomock.Any()).
					Times(1).Return(&sdump.URLEndpoint{}, errors.New("could not fetch url"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			requestBody:        strings.NewReader(``),
			requestBodySize:    10,
		},
		{
			name: "http request body too large",
			mockFn: func(urlRepo *mocks.MockURLRepository, requestRepo *mocks.MockIngestRepository) {
				urlRepo.EXPECT().Get(gomock.Any(), gomock.Any()).
					Times(1).Return(&sdump.URLEndpoint{}, nil)
			},
			expectedStatusCode: http.StatusBadRequest,
			requestBody:        strings.NewReader(`{"name" : "Lanre", "occupation" :"Software"}`),
			requestBodySize:    10,
		},
		{
			name: "could not create ingestion",
			mockFn: func(urlRepo *mocks.MockURLRepository, requestRepo *mocks.MockIngestRepository) {
				urlRepo.EXPECT().Get(gomock.Any(), gomock.Any()).
					Times(1).Return(&sdump.URLEndpoint{}, nil)

				requestRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("could not insert into the database"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			requestBody:        strings.NewReader(`{"name" : "Lanre", "occupation" :"Software"}`),
			requestBodySize:    100,
		},
		{
			name: "ingested correctly",
			mockFn: func(urlRepo *mocks.MockURLRepository, requestRepo *mocks.MockIngestRepository) {
				urlRepo.EXPECT().Get(gomock.Any(), gomock.Any()).
					Times(1).Return(&sdump.URLEndpoint{}, nil)

				requestRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			expectedStatusCode: http.StatusAccepted,
			requestBody:        strings.NewReader(`{"name" : "Lanre", "occupation" :"Software"}`),
			requestBodySize:    100,
		},
	}

	for _, v := range tt {
		t.Run(v.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodPost, "/", v.requestBody)

			logrus.SetOutput(io.Discard)

			logger := logrus.WithField("module", "test")

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			urlRepo := mocks.NewMockURLRepository(ctrl)

			requestRepo := mocks.NewMockIngestRepository(ctrl)

			v.mockFn(urlRepo, requestRepo)

			u := &urlHandler{
				logger: logger,
				cfg: config.Config{
					HTTP: config.HTTPConfig{
						MaxRequestBodySize: v.requestBodySize,
					},
				},
				urlRepo:    urlRepo,
				ingestRepo: requestRepo,
				sseServer:  sse.New(),
			}

			u.ingest(recorder, req)

			require.Equal(t, v.expectedStatusCode, recorder.Result().StatusCode)
			verifyMatch(t, recorder)
		})
	}
}
