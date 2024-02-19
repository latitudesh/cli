package servers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewServerScheduleDeletionParams creates a new ServerScheduleDeletionParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewServerScheduleDeletionParams() *ServerScheduleDeletionParams {
	return &ServerScheduleDeletionParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewServerScheduleDeletionParamsWithTimeout creates a new ServerScheduleDeletionParams object
// with the ability to set a timeout on a request.
func NewServerScheduleDeletionParamsWithTimeout(timeout time.Duration) *ServerScheduleDeletionParams {
	return &ServerScheduleDeletionParams{
		timeout: timeout,
	}
}

// NewServerScheduleDeletionParamsWithContext creates a new ServerScheduleDeletionParams object
// with the ability to set a context for a request.
func NewServerScheduleDeletionParamsWithContext(ctx context.Context) *ServerScheduleDeletionParams {
	return &ServerScheduleDeletionParams{
		Context: ctx,
	}
}

// NewServerScheduleDeletionParamsWithHTTPClient creates a new ServerScheduleDeletionParams object
// with the ability to set a custom HTTPClient for a request.
func NewServerScheduleDeletionParamsWithHTTPClient(client *http.Client) *ServerScheduleDeletionParams {
	return &ServerScheduleDeletionParams{
		HTTPClient: client,
	}
}

/*
ServerScheduleDeletionParams contains all the parameters to send to the API endpoint

	for the server schedule deletion operation.

	Typically these are written to a http.Request.
*/
type ServerScheduleDeletionParams struct {

	// ServerID.
	ServerID string `json:"id"`

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the server schedule deletion params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *ServerScheduleDeletionParams) WithDefaults() *ServerScheduleDeletionParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the server schedule deletion params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *ServerScheduleDeletionParams) SetDefaults() {
	val := ServerScheduleDeletionParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the server schedule deletion params
func (o *ServerScheduleDeletionParams) WithTimeout(timeout time.Duration) *ServerScheduleDeletionParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the server schedule deletion params
func (o *ServerScheduleDeletionParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the server schedule deletion params
func (o *ServerScheduleDeletionParams) WithContext(ctx context.Context) *ServerScheduleDeletionParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the server schedule deletion params
func (o *ServerScheduleDeletionParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the server schedule deletion params
func (o *ServerScheduleDeletionParams) WithHTTPClient(client *http.Client) *ServerScheduleDeletionParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the server schedule deletion params
func (o *ServerScheduleDeletionParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithServerID adds the serverID to the server schedule deletion params
func (o *ServerScheduleDeletionParams) WithServerID(serverID string) *ServerScheduleDeletionParams {
	o.SetServerID(serverID)
	return o
}

// SetServerID adds the serverId to the server schedule deletion params
func (o *ServerScheduleDeletionParams) SetServerID(serverID string) {
	o.ServerID = serverID
}

// WriteToRequest writes these params to a swagger request
func (o *ServerScheduleDeletionParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param server_id
	if err := r.SetPathParam("server_id", o.ServerID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
