package sdump

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type RequestDefinition struct {
	Body      string      `mapstructure:"body" json:"body,omitempty"`
	Query     string      `json:"query,omitempty"`
	Headers   http.Header `json:"headers,omitempty"`
	IPAddress net.IP      `json:"ip_address,omitempty" bson:"ip_address"`
	Size      int64       `json:"size,omitempty"`
	Method    string      `json:"method,omitempty"`
	Path      string      `json:"path,omitempty"`
}

type ResponseDefinition struct {
	Body       string      `json:"body,omitempty"`
	Headers    http.Header `json:"headers,omitempty"`
	StatusCode int         `json:"status_code,omitempty"`
	Size       int64       `json:"size,omitempty"`
}

type IngestHTTPRequest struct {
	ID      uuid.UUID         `bun:"type:uuid,default:uuid_generate_v4()" json:"id,omitempty" mapstructure:"id"`
	UrlID   uuid.UUID         `json:"url_id,omitempty"`
	Request RequestDefinition `json:"request,omitempty"`

	// No need to store content type, it will always be application/json

	CreatedAt time.Time  `bun:",nullzero,notnull,default:current_timestamp" json:"created_at,omitempty" bson:"created_at" mapstructure:"created_at"`
	UpdatedAt time.Time  `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at,omitempty" bson:"updated_at" mapstructure:"updated_at"`
	DeletedAt *time.Time `bun:",soft_delete,nullzero" json:"-,omitempty" bson:"deleted_at" mapstructure:"deleted_at"`

	bun.BaseModel `bun:"table:ingests"`
}

type DeleteIngestedRequestOptions struct {
	Before         time.Time
	UseSoftDeletes bool
}

type IngestRepository interface {
	Create(context.Context, *IngestHTTPRequest) error
	Delete(context.Context, *DeleteIngestedRequestOptions) error
}
