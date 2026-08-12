package httpapi

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"fleetview/internal/service"
	"fleetview/internal/transport/dto"
)


type settingsRequest struct {
	Theme              *string `json:"theme"`
	SortKey            *string `json:"sortKey"`
	SortDirection      *string `json:"sortDirection"`
	SpeedUnit          *string `json:"speedUnit"`
	MapType            *string `json:"mapType"`
	ShowOfflineDevices *bool   `json:"showOfflineDevices"`
	ClusterMarkers     *bool   `json:"clusterMarkers"`
	ShowTrails         *bool   `json:"showTrails"`
	AnimateMarkers     *bool   `json:"animateMarkers"`
	AutoFitBounds      *bool   `json:"autoFitBounds"`
	RefreshSeconds     *int    `json:"refreshSeconds"`
}

func (r settingsRequest) toPatch() service.SettingsPatch {
	return service.SettingsPatch{
		Theme:              r.Theme,
		SortKey:            r.SortKey,
		SortDirection:      r.SortDirection,
		SpeedUnit:          r.SpeedUnit,
		MapType:            r.MapType,
		ShowOfflineDevices: r.ShowOfflineDevices,
		ClusterMarkers:     r.ClusterMarkers,
		ShowTrails:         r.ShowTrails,
		AnimateMarkers:     r.AnimateMarkers,
		AutoFitBounds:      r.AutoFitBounds,
		RefreshSeconds:     r.RefreshSeconds,
	}
}


type devicePreferenceRequest struct {
	Hidden      *bool   `json:"hidden"`
	DisplayName *string `json:"displayName"`
	MarkerIcon  *string `json:"markerIcon"`
	MarkerColor *string `json:"markerColor"`
	Pinned      *bool   `json:"pinned"`
	SortIndex   *int    `json:"sortIndex"`
	Notes       *string `json:"notes"`
}

func (r devicePreferenceRequest) toPatch() service.DevicePreferencePatch {
	return service.DevicePreferencePatch{
		Hidden:      r.Hidden,
		DisplayName: r.DisplayName,
		MarkerIcon:  r.MarkerIcon,
		MarkerColor: r.MarkerColor,
		Pinned:      r.Pinned,
		SortIndex:   r.SortIndex,
		Notes:       r.Notes,
	}
}

type reorderRequest struct {
	DeviceIDs []string `json:"deviceIds"`
}


func (h *Handlers) GetPreferences(c *gin.Context) {
	prefs, err := h.prefs.Get(c.Request.Context(), UserIDOf(c))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromPreferences(prefs))
}


func (h *Handlers) UpdateSettings(c *gin.Context) {
	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "The request body could not be parsed.", "")
		return
	}

	settings, err := h.prefs.UpdateSettings(c.Request.Context(), UserIDOf(c), req.toPatch())
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	h.notifyFleetChanged()
	c.JSON(http.StatusOK, dto.FromSettings(settings))
}


func (h *Handlers) UpsertDevicePreference(c *gin.Context) {
	var req devicePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "The request body could not be parsed.", "")
		return
	}

	pref, err := h.prefs.UpsertDevicePreference(c.Request.Context(), UserIDOf(c), c.Param("deviceId"), req.toPatch())
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	h.notifyFleetChanged()
	c.JSON(http.StatusOK, dto.FromDevicePreference(pref))
}


func (h *Handlers) DeleteDevicePreference(c *gin.Context) {
	if err := h.prefs.DeleteDevicePreference(c.Request.Context(), UserIDOf(c), c.Param("deviceId")); err != nil {
		respondError(c, h.log, err)
		return
	}
	h.notifyFleetChanged()
	c.Status(http.StatusNoContent)
}


func (h *Handlers) ReorderDevices(c *gin.Context) {
	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "The request body could not be parsed.", "")
		return
	}
	if len(req.DeviceIDs) == 0 {
		writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Provide at least one device id.", "deviceIds")
		return
	}

	if err := h.prefs.Reorder(c.Request.Context(), UserIDOf(c), req.DeviceIDs); err != nil {
		respondError(c, h.log, err)
		return
	}
	h.notifyFleetChanged()
	c.Status(http.StatusNoContent)
}


func (h *Handlers) ResetPreferences(c *gin.Context) {
	if err := h.prefs.Reset(c.Request.Context(), UserIDOf(c)); err != nil {
		respondError(c, h.log, err)
		return
	}
	h.notifyFleetChanged()
	c.Status(http.StatusNoContent)
}


func (h *Handlers) UploadIcon(c *gin.Context) {
	maxBytes := h.cfg.Icons.MaxBytes

	
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes+4096)

	header, err := c.FormFile("icon")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_upload",
			`Attach an image file in the "icon" form field.`, "icon")
		return
	}
	if header.Size > maxBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "file_too_large",
			"The image is larger than the allowed size.", "icon")
		return
	}

	file, err := header.Open()
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if int64(len(data)) > maxBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "file_too_large",
			"The image is larger than the allowed size.", "icon")
		return
	}

	pref, err := h.prefs.SaveIcon(c.Request.Context(), UserIDOf(c), c.Param("deviceId"),
		header.Header.Get("Content-Type"), data)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	h.notifyFleetChanged()
	c.JSON(http.StatusOK, dto.FromDevicePreference(pref))
}


func (h *Handlers) DeleteIcon(c *gin.Context) {
	pref, err := h.prefs.DeleteIcon(c.Request.Context(), UserIDOf(c), c.Param("deviceId"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	h.notifyFleetChanged()
	c.JSON(http.StatusOK, dto.FromDevicePreference(pref))
}


func (h *Handlers) GetIcon(c *gin.Context) {
	icon, err := h.prefs.Icon(c.Request.Context(), c.Param("iconId"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	
	c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; sandbox")
	c.Data(http.StatusOK, icon.ContentType, icon.Data)
}


func (h *Handlers) notifyFleetChanged() {
	if h.hub == nil || h.hub.ClientCount() == 0 {
		return
	}
	h.hub.Publish(service.EventFleetUpdated, nil)
}
