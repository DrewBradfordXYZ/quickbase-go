# The legacy XML API

The JSON API doesn't cover everything QuickBase can do. The `xml` package reaches the rest (roles, groups, code pages, app variables, webhooks, CSV imports, HTML generation) through QuickBase's older XML API, sharing the main client's authentication, retries and throttling.

> QuickBase calls the XML API legacy and may retire it. Prefer a JSON method where one exists.

```go
import (
	"github.com/DrewBradfordXYZ/quickbase-go/v3"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/xml"
)

x := xml.New(client)

roles, err := x.GetRoleInfo(ctx, "bqxyz123")
if err != nil {
	var xmlErr *xml.Error
	if errors.As(err, &xmlErr) {
		log.Printf("XML error %d: %s", xmlErr.Code, xmlErr.Text)
	}
	return
}
for _, role := range roles.Roles {
	fmt.Println(role.ID, role.Name, role.Access.Description)
}

page, _ := x.GetDBPage(ctx, "bqxyz123", "dashboard.html")
```

`xml.IsUnauthorized(err)` and `xml.IsNotFound(err)` test for the common error codes.

## Rate limits

The XML API allows **10 requests per second per table**, separately from the JSON API's 100 per 10 seconds per user token. QuickBase enforces it dynamically, so it may not return a 429 right away. The client retries 429s the same way for both APIs; `WithProactiveThrottle` is sized for the JSON limit.

## Schema aliases

Give the XML client the same [schema](schema-aliases.md) and it accepts table aliases, and its results gain lookups by alias:

```go
schema := core.NewSchema().
	Table("projects", "bqxyz123").
	Field("name", 6).
	Field("status", 7).
	Table("tasks", "bqabc456").
	Field("title", 6).
	Build()
x := xml.New(client, xml.WithSchema(core.ResolveSchema(schema)))

count, _ := x.GetNumRecords(ctx, "projects")

record, _ := x.GetRecordInfo(ctx, "projects", 123)
fmt.Println(record.Field("name").Value, record.FieldByID(7).Value)

dbs, _ := x.GrantedDBs(ctx, xml.GrantedDBsOptions{})
fmt.Println(dbs.Database("projects").Name)

dtm, _ := x.GetAppDTMInfo(ctx, "bqxyz123")
fmt.Println(dtm.Table("tasks").LastModifiedTime)

sch, _ := x.GetSchema(ctx, "projects")
fmt.Println(sch.Field("name"), sch.ChildTable("tasks"))
```

| Result | Lookups (alias, or ID without a schema) |
|---|---|
| `GetRecordInfoResult` | `Field(key)`, `FieldByID(id)` |
| `GrantedDBsResult` | `Database(key)` |
| `GetAppDTMInfoResult` | `Table(key)` |
| `SchemaResult` | `Field(key)`, `FieldByID(id)`, `ChildTable(key)` |

## Methods

**App Discovery:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GrantedDBs(ctx, opts)` | API_GrantedDBs | List all apps/tables user can access |
| `FindDBByName(ctx, name, parentsOnly)` | API_FindDBByName | Find an app by name |
| `GetDBInfo(ctx, dbid)` | API_GetDBInfo | Get app/table metadata (record count, manager, timestamps) |
| `GetNumRecords(ctx, tableId)` | API_GetNumRecords | Get total record count for a table |

**Role Management:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetRoleInfo(ctx, appId)` | API_GetRoleInfo | Get all roles defined in an application |
| `UserRoles(ctx, appId)` | API_UserRoles | Get all users and their role assignments |
| `GetUserRole(ctx, appId, userId, includeGroups)` | API_GetUserRole | Get roles for a specific user |
| `AddUserToRole(ctx, appId, userId, roleId)` | API_AddUserToRole | Assign a user to a role |
| `RemoveUserFromRole(ctx, appId, userId, roleId)` | API_RemoveUserFromRole | Remove a user from a role |
| `ChangeUserRole(ctx, appId, userId, currentRole, newRole)` | API_ChangeUserRole | Change a user's role |

**User Management:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetUserInfo(ctx, email)` | API_GetUserInfo | Get user info by email address |
| `ProvisionUser(ctx, appId, email, firstName, lastName, roleId)` | API_ProvisionUser | Create a new unregistered user |
| `SendInvitation(ctx, appId, userId, userText)` | API_SendInvitation | Send invitation email to a user |
| `ChangeManager(ctx, appId, newManagerEmail)` | API_ChangeManager | Change the app manager |
| `ChangeRecordOwner(ctx, tableId, recordId, newOwner)` | API_ChangeRecordOwner | Change record owner |

**Group Management:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `CreateGroup(ctx, name, description, accountId)` | API_CreateGroup | Create a new group |
| `DeleteGroup(ctx, groupId)` | API_DeleteGroup | Delete a group |
| `CopyGroup(ctx, groupId, name, description, accountId)` | API_CopyGroup | Copy a group |
| `ChangeGroupInfo(ctx, groupId, name, description, accountId)` | API_ChangeGroupInfo | Update group name/description |
| `GetUsersInGroup(ctx, groupId, includeManagers)` | API_GetUsersInGroup | Get users and managers in a group |
| `AddUserToGroup(ctx, groupId, userId, allowAdminAccess)` | API_AddUserToGroup | Add a user to a group |
| `RemoveUserFromGroup(ctx, groupId, userId)` | API_RemoveUserFromGroup | Remove a user from a group |
| `GetGroupRole(ctx, appId, groupId)` | API_GetGroupRole | Get roles assigned to a group |
| `AddGroupToRole(ctx, appId, groupId, roleId)` | API_AddGroupToRole | Assign a group to a role |
| `RemoveGroupFromRole(ctx, appId, groupId, roleId, allRoles)` | API_RemoveGroupFromRole | Remove a group from a role |
| `GrantedGroups(ctx, userId, adminOnly)` | API_GrantedGroups | Get groups a user belongs to |
| `GrantedDBsForGroup(ctx, groupId)` | API_GrantedDBsForGroup | Get apps a group can access |

**App Metadata:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetAppDTMInfo(ctx, appId)` | API_GetAppDTMInfo | Get modification timestamps (fast, no auth required) |
| `GetAncestorInfo(ctx, appId)` | API_GetAncestorInfo | Get app copy/template lineage info |

**Application Variables:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetDBVar(ctx, appId, varName)` | API_GetDBVar | Get an application variable value |
| `SetDBVar(ctx, appId, varName, value)` | API_SetDBVar | Set an application variable value |

**Code Pages:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetDBPage(ctx, appId, pageIdOrName)` | API_GetDBPage | Get stored code page content |
| `AddReplaceDBPage(ctx, appId, pageName, pageId, pageType, pageBody)` | API_AddReplaceDBPage | Create or update a code page |

**Field Management:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `FieldAddChoices(ctx, tableId, fieldId, choices)` | API_FieldAddChoices | Add choices to a multiple-choice field |
| `FieldRemoveChoices(ctx, tableId, fieldId, choices)` | API_FieldRemoveChoices | Remove choices from a field |
| `SetKeyField(ctx, tableId, fieldId)` | API_SetKeyField | Set the key field for a table |

**Schema Information:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetSchema(ctx, dbid)` | API_GetSchema | Get comprehensive app/table metadata |

**Record Information:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `DoQueryCount(ctx, tableId, query)` | API_DoQueryCount | Get count of matching records (no data fetch) |
| `GetRecordInfo(ctx, tableId, recordId)` | API_GetRecordInfo | Get record with field metadata |
| `GetRecordInfoByKey(ctx, tableId, keyValue)` | API_GetRecordInfo | Get record by key field value |

**Record Operations:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `CopyMasterDetail(ctx, tableId, opts)` | API_CopyMasterDetail | Copy a master record with its detail records |
| `ImportFromCSV(ctx, tableId, opts)` | API_ImportFromCSV | Bulk import/update records from CSV data |
| `RunImport(ctx, tableId, importId)` | API_RunImport | Execute a saved import definition |

**Webhooks:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `WebhooksCreate(ctx, tableId, opts)` | API_Webhooks_Create | Create a webhook |
| `WebhooksEdit(ctx, tableId, webhookId, opts)` | API_Webhooks_Edit | Edit a webhook |
| `WebhooksDelete(ctx, tableId, webhookId)` | API_Webhooks_Delete | Delete a webhook |
| `WebhooksActivate(ctx, tableId, webhookId)` | API_Webhooks_Activate | Activate a webhook |
| `WebhooksDeactivate(ctx, tableId, webhookId)` | API_Webhooks_Deactivate | Deactivate a webhook |
| `WebhooksCopy(ctx, tableId, webhookId, name)` | API_Webhooks_Copy | Copy a webhook |

**HTML Generation:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GenAddRecordForm(ctx, tableId, fields)` | API_GenAddRecordForm | Generate HTML form for adding a record |
| `GenResultsTable(ctx, tableId, opts)` | API_GenResultsTable | Generate HTML/JS/CSV table of query results |
| `GetRecordAsHTML(ctx, tableId, opts)` | API_GetRecordAsHTML | Get a record rendered as HTML |

**Authentication:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `SignOut(ctx)` | API_SignOut | Clear ticket cookie (browser-focused) |

## Which API has what

| XML-Only (Implemented) | JSON-Only |
|------------------------|-----------|
| Roles & role assignments | Relationships |
| Group management (create, delete, copy, membership, roles) | Solutions (app packaging) |
| Application variables (DBVars) | Platform analytics |
| Code pages (get, add/replace) | Audit logs |
| User provisioning (provision, invite) | Field usage statistics |
| Field choice management | Document templates |
| Record ownership changes | |
| Webhooks (create, edit, delete, activate/deactivate, copy) | |
| CSV import (ImportFromCSV, RunImport) | |
| Copy master/detail records | |
| App metadata (GetAppDTMInfo, GetAncestorInfo) | |
| HTML generation (forms, tables, record views) | |

A fuller comparison, including what neither SDK implements yet: [xml-api-reference.md](xml-api-reference.md).
