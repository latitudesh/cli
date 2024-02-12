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

// NewDeleteProjectSSHKeyParams creates a new DeleteProjectSSHKeyParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewDeleteProjectSSHKeyParams() *DeleteProjectSSHKeyParams {
	return &DeleteProjectSSHKeyParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewDeleteProjectSSHKeyParamsWithTimeout creates a new DeleteProjectSSHKeyParams object
// with the ability to set a timeout on a request.
func NewDeleteProjectSSHKeyParamsWithTimeout(timeout time.Duration) *DeleteProjectSSHKeyParams {
	return &DeleteProjectSSHKeyParams{
		timeout: timeout,
	}
}

// NewDeleteProjectSSHKeyParamsWithContext creates a new DeleteProjectSSHKeyParams object
// with the ability to set a context for a request.
func NewDeleteProjectSSHKeyParamsWithContext(ctx context.Context) *DeleteProjectSSHKeyParams {
	return &DeleteProjectSSHKeyParams{
		Context: ctx,
	}
}

// NewDeleteProjectSSHKeyParamsWithHTTPClient creates a new DeleteProjectSSHKeyParams object
// with the ability to set a custom HTTPClient for a request.
func NewDeleteProjectSSHKeyParamsWithHTTPClient(client *http.Client) *DeleteProjectSSHKeyParams {
	return &DeleteProjectSSHKeyParams{
		HTTPClient: client,
	}
}

/*
DeleteProjectSSHKeyParams contains all the parameters to send to the API endpoint

	for the delete project ssh key operation.

	Typically these are written to a http.Request.
*/
type DeleteProjectSSHKeyParams struct {
	ProjectIDOrSlug string `json:"project"`
	SSHKeyID        string `json:"id"`

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the delete project ssh key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteProjectSSHKeyParams) WithDefaults() *DeleteProjectSSHKeyParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the delete project ssh key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteProjectSSHKeyParams) SetDefaults() {
	val := DeleteProjectSSHKeyParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) WithTimeout(timeout time.Duration) *DeleteProjectSSHKeyParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) WithContext(ctx context.Context) *DeleteProjectSSHKeyParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) WithHTTPClient(client *http.Client) *DeleteProjectSSHKeyParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithProjectIDOrSlug adds the projectIDOrSlug to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) WithProjectIDOrSlug(projectIDOrSlug string) *DeleteProjectSSHKeyParams {
	o.SetProjectIDOrSlug(projectIDOrSlug)
	return o
}

// SetProjectIDOrSlug adds the projectIdOrSlug to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) SetProjectIDOrSlug(projectIDOrSlug string) {
	o.ProjectIDOrSlug = projectIDOrSlug
}

// WithSSHKeyID adds the sSHKeyID to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) WithSSHKeyID(sSHKeyID string) *DeleteProjectSSHKeyParams {
	o.SetSSHKeyID(sSHKeyID)
	return o
}

// SetSSHKeyID adds the sshKeyId to the delete project ssh key params
func (o *DeleteProjectSSHKeyParams) SetSSHKeyID(sSHKeyID string) {
	o.SSHKeyID = sSHKeyID
}

// WriteToRequest writes these params to a swagger request
func (o *DeleteProjectSSHKeyParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

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
