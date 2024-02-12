package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"

	apierrors "github.com/latitudesh/lsh/internal/api/errors"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/models"
)

// CreateProjectReader is a Reader for the CreateProject structure.
type CreateProjectReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *CreateProjectReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 201:
		result := NewCreateProjectCreated()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := apierrors.NewBadRequest()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 401:
		result := apierrors.NewUnauthorized()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := apierrors.NewForbidden()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 422:
		result := apierrors.NewUnprocessableEntity()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("[POST /projects] create-project", response, response.Code())
	}
}

// NewCreateProjectCreated creates a CreateProjectCreated with default headers values
func NewCreateProjectCreated() *CreateProjectCreated {
	return &CreateProjectCreated{}
}

/*
CreateProjectCreated describes a response with status code 201, with default header values.

Success
*/
type CreateProjectCreated struct {
	Payload *CreateProjectCreatedBody
}

// IsSuccess returns true when this create project created response has a 2xx status code
func (o *CreateProjectCreated) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this create project created response has a 3xx status code
func (o *CreateProjectCreated) IsRedirect() bool {
	return false
}

// IsClientError returns true when this create project created response has a 4xx status code
func (o *CreateProjectCreated) IsClientError() bool {
	return false
}

// IsServerError returns true when this create project created response has a 5xx status code
func (o *CreateProjectCreated) IsServerError() bool {
	return false
}

// IsCode returns true when this create project created response a status code equal to that given
func (o *CreateProjectCreated) IsCode(code int) bool {
	return code == 201
}

// Code gets the status code for the create project created response
func (o *CreateProjectCreated) Code() int {
	return 201
}

func (o *CreateProjectCreated) Error() string {
	return fmt.Sprintf("[POST /projects][%d] createProjectCreated  %+v", 201, o.Payload)
}

func (o *CreateProjectCreated) String() string {
	return fmt.Sprintf("[POST /projects][%d] createProjectCreated  %+v", 201, o.Payload)
}

func (o *CreateProjectCreated) GetPayload() *CreateProjectCreatedBody {
	return o.Payload
}

func (o *CreateProjectCreated) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{o.Payload.Data}
}

func (o *CreateProjectCreated) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(CreateProjectCreatedBody)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

/*
CreateProjectBody create project body
swagger:model CreateProjectBody
*/
type CreateProjectBody struct {

	// data
	Data *CreateProjectParamsBodyData `json:"data,omitempty"`
}

// Validate validates this create project body
func (o *CreateProjectBody) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateData(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *CreateProjectBody) validateData(formats strfmt.Registry) error {
	if swag.IsZero(o.Data) { // not required
		return nil
	}

	if o.Data != nil {
		if err := o.Data.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this create project body based on the context it is used
func (o *CreateProjectBody) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateData(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *CreateProjectBody) contextValidateData(ctx context.Context, formats strfmt.Registry) error {

	if o.Data != nil {

		if swag.IsZero(o.Data) { // not required
			return nil
		}

		if err := o.Data.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (o *CreateProjectBody) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *CreateProjectBody) UnmarshalBinary(b []byte) error {
	var res CreateProjectBody
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
CreateProjectCreatedBody create project created body
swagger:model CreateProjectCreatedBody
*/
type CreateProjectCreatedBody struct {

	// data
	Data *models.Project `json:"data,omitempty"`
}

// Validate validates this create project created body
func (o *CreateProjectCreatedBody) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateData(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *CreateProjectCreatedBody) validateData(formats strfmt.Registry) error {
	if swag.IsZero(o.Data) { // not required
		return nil
	}

	if o.Data != nil {
		if err := o.Data.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("createProjectCreated" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("createProjectCreated" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this create project created body based on the context it is used
func (o *CreateProjectCreatedBody) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateData(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *CreateProjectCreatedBody) contextValidateData(ctx context.Context, formats strfmt.Registry) error {

	if o.Data != nil {

		if swag.IsZero(o.Data) { // not required
			return nil
		}

		if err := o.Data.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("createProjectCreated" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("createProjectCreated" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (o *CreateProjectCreatedBody) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *CreateProjectCreatedBody) UnmarshalBinary(b []byte) error {
	var res CreateProjectCreatedBody
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
CreateProjectParamsBodyData create project params body data
swagger:model CreateProjectParamsBodyData
*/
type CreateProjectParamsBodyData struct {

	// attributes
	Attributes *CreateProjectParamsBodyDataAttributes `json:"attributes,omitempty"`

	// type
	// Required: true
	// Enum: [projects]
	Type *string `json:"type"`
}

// Validate validates this create project params body data
func (o *CreateProjectParamsBodyData) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateAttributes(formats); err != nil {
		res = append(res, err)
	}

	if err := o.validateType(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *CreateProjectParamsBodyData) validateAttributes(formats strfmt.Registry) error {
	if swag.IsZero(o.Attributes) { // not required
		return nil
	}

	if o.Attributes != nil {
		if err := o.Attributes.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data" + "." + "attributes")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data" + "." + "attributes")
			}
			return err
		}
	}

	return nil
}

var createProjectParamsBodyDataTypeTypePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["projects"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		createProjectParamsBodyDataTypeTypePropEnum = append(createProjectParamsBodyDataTypeTypePropEnum, v)
	}
}

const (

	// CreateProjectParamsBodyDataTypeProjects captures enum value "projects"
	CreateProjectParamsBodyDataTypeProjects string = "projects"
)

// prop value enum
func (o *CreateProjectParamsBodyData) validateTypeEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, createProjectParamsBodyDataTypeTypePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (o *CreateProjectParamsBodyData) validateType(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"type", "body", o.Type); err != nil {
		return err
	}

	// value enum
	if err := o.validateTypeEnum("body"+"."+"data"+"."+"type", "body", *o.Type); err != nil {
		return err
	}

	return nil
}

// ContextValidate validate this create project params body data based on the context it is used
func (o *CreateProjectParamsBodyData) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateAttributes(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *CreateProjectParamsBodyData) contextValidateAttributes(ctx context.Context, formats strfmt.Registry) error {

	if o.Attributes != nil {

		if swag.IsZero(o.Attributes) { // not required
			return nil
		}

		if err := o.Attributes.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data" + "." + "attributes")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data" + "." + "attributes")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (o *CreateProjectParamsBodyData) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *CreateProjectParamsBodyData) UnmarshalBinary(b []byte) error {
	var res CreateProjectParamsBodyData
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
CreateProjectParamsBodyDataAttributes create project params body data attributes
swagger:model CreateProjectParamsBodyDataAttributes
*/
type CreateProjectParamsBodyDataAttributes struct {
	Description      string `json:"description,omitempty"`
	Environment      string `json:"environment,omitempty"`
	Name             string `json:"name"`
	ProvisioningType string `json:"provisioning_type"`
}

// Validate validates this create project params body data attributes
func (o *CreateProjectParamsBodyDataAttributes) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateEnvironment(formats); err != nil {
		res = append(res, err)
	}

	if err := o.validateName(formats); err != nil {
		res = append(res, err)
	}

	if err := o.validateProvisioningType(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

var createProjectParamsBodyDataAttributesTypeEnvironmentPropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["Development","Staging","Production"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		createProjectParamsBodyDataAttributesTypeEnvironmentPropEnum = append(createProjectParamsBodyDataAttributesTypeEnvironmentPropEnum, v)
	}
}

const (

	// CreateProjectParamsBodyDataAttributesEnvironmentDevelopment captures enum value "Development"
	CreateProjectParamsBodyDataAttributesEnvironmentDevelopment string = "Development"

	// CreateProjectParamsBodyDataAttributesEnvironmentStaging captures enum value "Staging"
	CreateProjectParamsBodyDataAttributesEnvironmentStaging string = "Staging"

	// CreateProjectParamsBodyDataAttributesEnvironmentProduction captures enum value "Production"
	CreateProjectParamsBodyDataAttributesEnvironmentProduction string = "Production"
)

// prop value enum
func (o *CreateProjectParamsBodyDataAttributes) validateEnvironmentEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, createProjectParamsBodyDataAttributesTypeEnvironmentPropEnum, true); err != nil {
		return err
	}
	return nil
}

func (o *CreateProjectParamsBodyDataAttributes) validateEnvironment(formats strfmt.Registry) error {
	if swag.IsZero(o.Environment) { // not required
		return nil
	}

	// value enum
	if err := o.validateEnvironmentEnum("body"+"."+"data"+"."+"attributes"+"."+"environment", "body", o.Environment); err != nil {
		return err
	}

	return nil
}

func (o *CreateProjectParamsBodyDataAttributes) validateName(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"attributes"+"."+"name", "body", o.Name); err != nil {
		return err
	}

	return nil
}

var createProjectParamsBodyDataAttributesTypeProvisioningTypePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["reserved","on_demand"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		createProjectParamsBodyDataAttributesTypeProvisioningTypePropEnum = append(createProjectParamsBodyDataAttributesTypeProvisioningTypePropEnum, v)
	}
}

const (

	// CreateProjectParamsBodyDataAttributesProvisioningTypeReserved captures enum value "reserved"
	CreateProjectParamsBodyDataAttributesProvisioningTypeReserved string = "reserved"

	// CreateProjectParamsBodyDataAttributesProvisioningTypeOnDemand captures enum value "on_demand"
	CreateProjectParamsBodyDataAttributesProvisioningTypeOnDemand string = "on_demand"
)

// prop value enum
func (o *CreateProjectParamsBodyDataAttributes) validateProvisioningTypeEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, createProjectParamsBodyDataAttributesTypeProvisioningTypePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (o *CreateProjectParamsBodyDataAttributes) validateProvisioningType(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"attributes"+"."+"provisioning_type", "body", o.ProvisioningType); err != nil {
		return err
	}

	// value enum
	if err := o.validateProvisioningTypeEnum("body"+"."+"data"+"."+"attributes"+"."+"provisioning_type", "body", o.ProvisioningType); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this create project params body data attributes based on context it is used
func (o *CreateProjectParamsBodyDataAttributes) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (o *CreateProjectParamsBodyDataAttributes) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *CreateProjectParamsBodyDataAttributes) UnmarshalBinary(b []byte) error {
	var res CreateProjectParamsBodyDataAttributes
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}
