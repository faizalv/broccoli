package sdkir

type HTTPMethod string

const (
	MethodGET    HTTPMethod = "GET"
	MethodPOST   HTTPMethod = "POST"
	MethodPUT    HTTPMethod = "PUT"
	MethodPATCH  HTTPMethod = "PATCH"
	MethodDELETE HTTPMethod = "DELETE"
)

type ParamLocation string

const (
	ParamPath   ParamLocation = "path"
	ParamQuery  ParamLocation = "query"
	ParamHeader ParamLocation = "header"
)

type Param struct {
	Name     string
	Location ParamLocation
	GoType   string
	Required bool
}

type Route struct {
	FuncName    string
	Method      HTTPMethod
	Path        string
	PathParams  []Param // ordered by occurrence in path
	QueryParams []Param
	BodyType    string
	RespType    string
	RespIsArray bool
}

type SDK struct {
	Routes []Route
}
