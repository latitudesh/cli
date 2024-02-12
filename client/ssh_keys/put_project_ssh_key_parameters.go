package ssh_keys

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewPutProjectSSHKeyParams creates a new PutProjectSSHKeyParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewPutProjectSSHKeyParams() *PutProjectSSHKeyParams {
	return &PutProjectSSHKeyParams{
		timeout: cr.DefaultTimeout,
		Body: PutProjectSSHKeyBody{
			Data: &PutProjectSSHKeyParamsBodyData{
				Type:       &sshKeysType,
				Attributes: &PutProjectSSHKeyParamsBodyDataAttributes{},
			},
		},
	}
}

// NewPutProjectSSHKeyParamsWithTimeout creates a new PutProjectSSHKeyParams object
// with the ability to set a timeout on a request.
func NewPutProjectSSHKeyParamsWithTimeout(timeout time.Duration) *PutProjectSSHKeyParams {
	return &PutProjectSSHKeyParams{
		timeout: timeout,
	}
}

// NewPutProjectSSHKeyParamsWithContext creates a new PutProjectSSHKeyParams object
// with the ability to set a context for a request.
func NewPutProjectSSHKeyParamsWithContext(ctx context.Context) *PutProjectSSHKeyParams {
	return &PutProjectSSHKeyParams{
		Context: ctx,
	}
}

// NewPutProjectSSHKeyParamsWithHTTPClient creates a new PutProjectSSHKeyParams object
// with the ability to set a custom HTTPClient for a request.
func NewPutProjectSSHKeyParamsWithHTTPClient(client *http.Client) *PutProjectSSHKeyParams {
	return &PutProjectSSHKeyParams{
		HTTPClient: client,
	}
}

/*
PutProjectSSHKeyParams contains all the parameters to send to the API endpoint

	for the put project ssh key operation.

	Typically these are written to a http.Request.
*/
type PutProjectSSHKeyParams struct {

	// Body.
	Body PutProjectSSHKeyBody

	// ProjectIDOrSlug.
	ProjectIDOrSlug string `json:"project"`

	// SSHKeyID.
	SSHKeyID string `json:"id"`

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the put project ssh key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutProjectSSHKeyParams) WithDefaults() *PutProjectSSHKeyParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the put project ssh key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutProjectSSHKeyParams) SetDefaults() {
	val := PutProjectSSHKeyParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the put project ssh key params
func (o *PutProjectSSHKeyParams) WithTimeout(timeout time.Duration) *PutProjectSSHKeyParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the put project ssh key params
func (o *PutProjectSSHKeyParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the put project ssh key params
func (o *PutProjectSSHKeyParams) WithContext(ctx context.Context) *PutProjectSSHKeyParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the put project ssh key params
func (o *PutProjectSSHKeyParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the put project ssh key params
func (o *PutProjectSSHKeyParams) WithHTTPClient(client *http.Client) *PutProjectSSHKeyParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the put project ssh key params
func (o *PutProjectSSHKeyParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the put project ssh key params
func (o *PutProjectSSHKeyParams) WithBody(body PutProjectSSHKeyBody) *PutProjectSSHKeyParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the put project ssh key params
func (o *PutProjectSSHKeyParams) SetBody(body PutProjectSSHKeyBody) {
	o.Body = body
}

// WithProjectIDOrSlug adds the projectIDOrSlug to the put project ssh key params
func (o *PutProjectSSHKeyParams) WithProjectIDOrSlug(projectIDOrSlug string) *PutProjectSSHKeyParams {
	o.SetProjectIDOrSlug(projectIDOrSlug)
	return o
}

// SetProjectIDOrSlug adds the projectIdOrSlug to the put project ssh key params
func (o *PutProjectSSHKeyParams) SetProjectIDOrSlug(projectIDOrSlug string) {
	o.ProjectIDOrSlug = projectIDOrSlug
}

// WithSSHKeyID adds the sSHKeyID to the put project ssh key params
func (o *PutProjectSSHKeyParams) WithSSHKeyID(sSHKeyID string) *PutProjectSSHKeyParams {
	o.SetSSHKeyID(sSHKeyID)
	return o
}

// SetSSHKeyID adds the sshKeyId to the put project ssh key params
func (o *PutProjectSSHKeyParams) SetSSHKeyID(sSHKeyID string) {
	o.SSHKeyID = sSHKeyID
}

// WriteToRequest writes these params to a swagger request
func (o *PutProjectSSHKeyParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if err := r.SetBodyParam(o.Body); err != nil {
		return err
	}

	// path param project_id_or_slug
	if err := r.SetPathParam("project_id_or_slug", o.ProjectIDOrSlug); err != nil {
		return err
	}

	// path param ssh_key_id
	if err := r.SetPathParam("ssh_key_id", o.SSHKeyID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
