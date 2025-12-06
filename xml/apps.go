package xml

import (
	"context"
	"encoding/xml"
	"fmt"
)

// GrantedDBInfo contains information about an accessible app or table.
type GrantedDBInfo struct {
	// DBID is the database/table ID
	DBID string

	// Name is the app/table name. Child tables appear as "AppName:TableName"
	Name string

	// AncestorAppID is the dbid of the app this was copied from (if any)
	AncestorAppID string

	// OldestAncestorAppID is the dbid of the original app in the copy chain
	OldestAncestorAppID string
}

// GrantedDBsResult contains the response from API_GrantedDBs.
type GrantedDBsResult struct {
	// Databases is the list of accessible apps and tables
	Databases []GrantedDBInfo
}

// GrantedDBsOptions configures the GrantedDBs call.
type GrantedDBsOptions struct {
	// AdminOnly returns only tables where the user has admin privileges
	AdminOnly bool

	// ExcludeParents excludes application-level dbids (returns only child tables)
	ExcludeParents bool

	// WithEmbeddedTables includes child table dbids (default true)
	WithEmbeddedTables *bool

	// IncludeAncestors includes ancestor/oldest ancestor info in results
	IncludeAncestors bool

	// RealmAppsOnly returns only apps in the current realm (not all accessible realms)
	RealmAppsOnly bool
}

// dbInfoXML is the XML structure for a database info entry.
type dbInfoXML struct {
	DBName              string `xml:"dbname"`
	DBID                string `xml:"dbid"`
	AncestorAppID       string `xml:"ancestorappid"`
	OldestAncestorAppID string `xml:"oldestancestorappid"`
}

// grantedDBsResponse is the XML response structure for API_GrantedDBs.
type grantedDBsResponse struct {
	BaseResponse
	Databases struct {
		DBInfo []dbInfoXML `xml:"dbinfo"`
	} `xml:"databases"`
}

// GrantedDBs returns a list of all apps and tables the user can access.
//
// By default, this returns apps across all realms the user has access to.
// Use options to filter the results (e.g., only current realm, only admin access).
//
// Child table names appear as "AppName:TableName" in the results.
//
// Example:
//
//	// Get all accessible apps in this realm only
//	result, err := xmlClient.GrantedDBs(ctx, xml.GrantedDBsOptions{
//	    RealmAppsOnly: true,
//	})
//	for _, db := range result.Databases {
//	    fmt.Printf("%s: %s\n", db.DBID, db.Name)
//	}
//
//	// Get only apps where user is admin, with copy lineage
//	result, err := xmlClient.GrantedDBs(ctx, xml.GrantedDBsOptions{
//	    AdminOnly:        true,
//	    IncludeAncestors: true,
//	})
//
// See: https://help.quickbase.com/docs/api-granteddbs
func (c *Client) GrantedDBs(ctx context.Context, opts GrantedDBsOptions) (*GrantedDBsResult, error) {
	inner := ""

	if opts.AdminOnly {
		inner += "<adminOnly>1</adminOnly>"
	}
	if opts.ExcludeParents {
		inner += "<excludeparents>1</excludeparents>"
	}
	if opts.WithEmbeddedTables != nil {
		if *opts.WithEmbeddedTables {
			inner += "<withembeddedtables>1</withembeddedtables>"
		} else {
			inner += "<withembeddedtables>0</withembeddedtables>"
		}
	}
	if opts.IncludeAncestors {
		inner += "<includeancestors>1</includeancestors>"
	}
	if opts.RealmAppsOnly {
		inner += "<realmAppsOnly>1</realmAppsOnly>"
	}

	body := buildRequest(inner)

	// GrantedDBs is invoked on db/main, not a specific dbid
	respBody, err := c.caller.DoXML(ctx, "main", "API_GrantedDBs", body)
	if err != nil {
		return nil, fmt.Errorf("API_GrantedDBs: %w", err)
	}

	var resp grantedDBsResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GrantedDBs response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	// Convert to result type
	databases := make([]GrantedDBInfo, len(resp.Databases.DBInfo))
	for i, db := range resp.Databases.DBInfo {
		databases[i] = GrantedDBInfo{
			DBID:                db.DBID,
			Name:                db.DBName,
			AncestorAppID:       db.AncestorAppID,
			OldestAncestorAppID: db.OldestAncestorAppID,
		}
	}

	return &GrantedDBsResult{
		Databases: databases,
	}, nil
}

// FindDBByNameResult contains the response from API_FindDBByName.
type FindDBByNameResult struct {
	// DBID is the database ID of the found app
	DBID string

	// Name is the app name (echoed from request)
	Name string
}

// findDBByNameResponse is the XML response structure for API_FindDBByName.
type findDBByNameResponse struct {
	BaseResponse
	DBID   string `xml:"dbid"`
	DBName string `xml:"dbname"`
}

// FindDBByName finds an app by its name.
//
// Quickbase searches only apps you have access to. Multiple apps can have
// the same name, but this returns only the first match.
//
// If the app has only one table, this returns the table dbid by default.
// Set parentsOnly=true to always get the app dbid.
//
// Example:
//
//	result, err := xmlClient.FindDBByName(ctx, "My Project App", true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("App DBID: %s\n", result.DBID)
//
// See: https://help.quickbase.com/docs/api-finddbbyname
func (c *Client) FindDBByName(ctx context.Context, name string, parentsOnly bool) (*FindDBByNameResult, error) {
	inner := "<dbname>" + xmlEscape(name) + "</dbname>"
	if parentsOnly {
		inner += "<ParentsOnly>1</ParentsOnly>"
	}
	body := buildRequest(inner)

	// FindDBByName is invoked on db/main
	respBody, err := c.caller.DoXML(ctx, "main", "API_FindDBByName", body)
	if err != nil {
		return nil, fmt.Errorf("API_FindDBByName: %w", err)
	}

	var resp findDBByNameResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_FindDBByName response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &FindDBByNameResult{
		DBID: resp.DBID,
		Name: resp.DBName,
	}, nil
}

// GetDBInfoResult contains the response from API_GetDBInfo.
type GetDBInfoResult struct {
	// Name is the app/table name
	Name string

	// LastRecModTime is when a record was last modified (Unix ms)
	LastRecModTime int64

	// LastModifiedTime is when the table structure was last modified (Unix ms)
	LastModifiedTime int64

	// CreatedTime is when the table was created (Unix ms)
	CreatedTime int64

	// NumRecords is the total record count
	NumRecords int

	// ManagerID is the unique ID of the table manager
	ManagerID string

	// ManagerName is the name of the table manager
	ManagerName string

	// TimeZone is the app's time zone string
	TimeZone string
}

// getDBInfoResponse is the XML response structure for API_GetDBInfo.
type getDBInfoResponse struct {
	BaseResponse
	DBName           string `xml:"dbname"`
	LastRecModTime   int64  `xml:"lastRecModTime"`
	LastModifiedTime int64  `xml:"lastModifiedTime"`
	CreatedTime      int64  `xml:"createdTime"`
	NumRecords       int    `xml:"numRecords"`
	ManagerID        string `xml:"mgrID"`
	ManagerName      string `xml:"mgrName"`
	TimeZone         string `xml:"time_zone"`
}

// GetDBInfo returns metadata about an app or table.
//
// This is useful for quick checks like:
//   - Has the table changed since last sync? (compare LastModifiedTime)
//   - How many records are in the table?
//   - Who is the manager?
//
// Example:
//
//	info, err := xmlClient.GetDBInfo(ctx, tableId)
//	fmt.Printf("Table: %s\n", info.Name)
//	fmt.Printf("Records: %d\n", info.NumRecords)
//	fmt.Printf("Manager: %s\n", info.ManagerName)
//
// See: https://help.quickbase.com/docs/api-getdbinfo
func (c *Client) GetDBInfo(ctx context.Context, dbid string) (*GetDBInfoResult, error) {
	body := buildRequest("")

	respBody, err := c.caller.DoXML(ctx, dbid, "API_GetDBInfo", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetDBInfo: %w", err)
	}

	var resp getDBInfoResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetDBInfo response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &GetDBInfoResult{
		Name:             resp.DBName,
		LastRecModTime:   resp.LastRecModTime,
		LastModifiedTime: resp.LastModifiedTime,
		CreatedTime:      resp.CreatedTime,
		NumRecords:       resp.NumRecords,
		ManagerID:        resp.ManagerID,
		ManagerName:      resp.ManagerName,
		TimeZone:         resp.TimeZone,
	}, nil
}

// GetNumRecords returns the total number of records in a table.
//
// This is a lightweight call that only returns the count.
// For counting records that match a query, use DoQueryCount instead.
//
// Example:
//
//	count, err := xmlClient.GetNumRecords(ctx, tableId)
//	fmt.Printf("Total records: %d\n", count)
//
// See: https://help.quickbase.com/docs/api-getnumrecords
func (c *Client) GetNumRecords(ctx context.Context, tableId string) (int, error) {
	body := buildRequest("")

	respBody, err := c.caller.DoXML(ctx, tableId, "API_GetNumRecords", body)
	if err != nil {
		return 0, fmt.Errorf("API_GetNumRecords: %w", err)
	}

	var resp struct {
		BaseResponse
		NumRecords int `xml:"num_records"`
	}
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return 0, fmt.Errorf("parsing API_GetNumRecords response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return 0, err
	}

	return resp.NumRecords, nil
}
