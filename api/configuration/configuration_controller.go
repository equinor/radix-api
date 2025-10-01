package configuration

import (
	"net/http"

	"github.com/equinor/radix-api/models"
)

const rootPath = "/configuration"

type configurationController struct {
	*models.DefaultController

	handler ConfigurationHandler
}

// NewConfigurationController Constructor
func NewConfigurationController(handler ConfigurationHandler) models.Controller {
	return &configurationController{
		handler: handler,
	}
}

// GetRoutes List the supported routes of this handler
func (c *configurationController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:                      rootPath + "/settings",
			Method:                    "GET",
			HandlerFunc:               c.GetSettings,
			AllowUnauthenticatedUsers: false,
		},
	}

	return routes
}

// GetSettings reveals the settings for the selected cluster environment
func (c *configurationController) GetSettings(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /configuration/settings settings getConfigurationSettings
	// ---
	// summary: Show the cluster environment settings
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        "$ref": "#/definitions/ConfigurationSettings"
	//   "500":
	//     description: "Internal Server Error"

	s, err := c.handler.GetSettings(r.Context())
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, s)
}
