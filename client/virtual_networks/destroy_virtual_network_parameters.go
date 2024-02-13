package virtual_networks

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewDestroyVirtualNetworkParams creates a new DestroyVirtualNetworkParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewDestroyVirtualNetworkParams() *DestroyVirtualNetworkParams {
	return &DestroyVirtualNetworkParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewDestroyVirtualNetworkParamsWithTimeout creates a new DestroyVirtualNetworkParams object
// with the ability to set a timeout on a request.
func NewDestroyVirtualNetworkParamsWithTimeout(timeout time.Duration) *DestroyVirtualNetworkParams {
	return &DestroyVirtualNetworkParams{
		timeout: timeout,
	}
}

// NewDestroyVirtualNetworkParamsWithContext creates a new DestroyVirtualNetworkParams object
// with the ability to set a context for a request.
func NewDestroyVirtualNetworkParamsWithContext(ctx context.Context) *DestroyVirtualNetworkParams {
	return &DestroyVirtualNetworkParams{
		Context: ctx,
	}
}

// NewDestroyVirtualNetworkParamsWithHTTPClient creates a new DestroyVirtualNetworkParams object
// with the ability to set a custom HTTPClient for a request.
func NewDestroyVirtualNetworkParamsWithHTTPClient(client *http.Client) *DestroyVirtualNetworkParams {
	return &DestroyVirtualNetworkParams{
		HTTPClient: client,
	}
}

/*
DestroyVirtualNetworkParams contains all the parameters to send to the API endpoint

	for the destroy virtual network operation.

	Typically these are written to a http.Request.
*/
type DestroyVirtualNetworkParams struct {
	ID string `json:"id"`

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the destroy virtual network params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DestroyVirtualNetworkParams) WithDefaults() *DestroyVirtualNetworkParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the destroy virtual network params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DestroyVirtualNetworkParams) SetDefaults() {
	val := DestroyVirtualNetworkParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) WithTimeout(timeout time.Duration) *DestroyVirtualNetworkParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) WithContext(ctx context.Context) *DestroyVirtualNetworkParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) WithHTTPClient(client *http.Client) *DestroyVirtualNetworkParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithID adds the id to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) WithID(id string) *DestroyVirtualNetworkParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the destroy virtual network params
func (o *DestroyVirtualNetworkParams) SetID(id string) {
	o.ID = id
}

// WriteToRequest writes these params to a swagger request
func (o *DestroyVirtualNetworkParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param id
	if err := r.SetPathParam("id", o.ID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
