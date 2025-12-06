# XML vs JSON API Reference

This document provides a comprehensive comparison of QuickBase's legacy XML API and the modern JSON RESTful API, identifying unique capabilities of each and their overlap.

> **Note:** The XML API is legacy and may be discontinued by QuickBase in the future. Use JSON API methods where possible.

## Summary

| API | Endpoint Count | Unique Capabilities |
|-----|----------------|---------------------|
| **JSON API** | 62 operations | Relationships, Solutions, Platform Analytics, Field Usage, Audit Logs |
| **XML API** | 71 endpoints | Roles, Groups, DBVars, Code Pages, Webhooks, User Provisioning |
| **Overlap** | ~20 operations | Apps, Tables, Fields, Records, Files, User Token Management |

## Rate Limits

The APIs have different rate limiting:

| API | Rate Limit | Scope | Enforcement |
|-----|------------|-------|-------------|
| **JSON API** | 100 requests / 10 seconds | Per user token | Fixed, returns 429 |
| **XML API** | 10 requests / second | Per table | Dynamic, may throttle |

The XML API's per-table limit means you can make 10 req/s to table A and 10 req/s to table B simultaneously. The SDK handles 429 responses with exponential backoff.

## Implemented Endpoints

These XML endpoints are available in the `xml` sub-package:

### App Discovery
| Method | XML Action | Description |
|--------|------------|-------------|
| `GrantedDBs(ctx, opts)` | `API_GrantedDBs` | List all apps/tables user can access |
| `FindDBByName(ctx, name, parentsOnly)` | `API_FindDBByName` | Find an app by name |
| `GetDBInfo(ctx, dbid)` | `API_GetDBInfo` | Get app/table metadata (record count, manager, timestamps) |
| `GetNumRecords(ctx, tableId)` | `API_GetNumRecords` | Get total record count for a table |

### Role Management
| Method | XML Action | Description |
|--------|------------|-------------|
| `GetRoleInfo(ctx, appId)` | `API_GetRoleInfo` | Get all roles defined in an application |
| `UserRoles(ctx, appId)` | `API_UserRoles` | Get all users and their role assignments |
| `GetUserRole(ctx, appId, userId, includeGroups)` | `API_GetUserRole` | Get roles for a specific user |
| `AddUserToRole(ctx, appId, userId, roleId)` | `API_AddUserToRole` | Assign a user to a role |
| `RemoveUserFromRole(ctx, appId, userId, roleId)` | `API_RemoveUserFromRole` | Remove a user from a role |
| `ChangeUserRole(ctx, appId, userId, currentRole, newRole)` | `API_ChangeUserRole` | Change a user's role |

### User Information
| Method | XML Action | Description |
|--------|------------|-------------|
| `GetUserInfo(ctx, email)` | `API_GetUserInfo` | Get user info by email address |

### Application Variables
| Method | XML Action | Description |
|--------|------------|-------------|
| `GetDBVar(ctx, appId, varName)` | `API_GetDBVar` | Get an application variable value |
| `SetDBVar(ctx, appId, varName, value)` | `API_SetDBVar` | Set an application variable value |

### Schema Information
| Method | XML Action | Description |
|--------|------------|-------------|
| `GetSchema(ctx, dbid)` | `API_GetSchema` | Get comprehensive app/table metadata |

### Record Information
| Method | XML Action | Description |
|--------|------------|-------------|
| `DoQueryCount(ctx, tableId, query)` | `API_DoQueryCount` | Get count of matching records without fetching data |
| `GetRecordInfo(ctx, tableId, recordId)` | `API_GetRecordInfo` | Get a record with full field metadata |
| `GetRecordInfoByKey(ctx, tableId, keyValue)` | `API_GetRecordInfo` | Get a record by key field value |

---

## JSON-Only Capabilities (Not Available in XML API)

These features are **only available in the JSON API**:

### Relationships
| JSON Endpoint | Description |
|---------------|-------------|
| `GET /tables/{tableId}/relationships` | Get all relationships for a table |
| `POST /tables/{tableId}/relationship` | Create a relationship |
| `POST /tables/{tableId}/relationship/{relationshipId}` | Update a relationship |
| `DELETE /tables/{tableId}/relationship/{relationshipId}` | Delete a relationship |

### Solutions (App Packaging)
| JSON Endpoint | Description |
|---------------|-------------|
| `POST /solutions` | Create a solution |
| `GET /solutions/{solutionId}` | Export a solution |
| `PUT /solutions/{solutionId}` | Update a solution |
| `GET /solutions/{solutionId}/torecord` | Export solution to record |
| `GET /solutions/fromrecord` | Create solution from record |
| `GET /solutions/{solutionId}/fromrecord` | Update solution from record |
| `PUT /solutions/{solutionId}/changeset` | List solution changes |
| `GET /solutions/{solutionId}/changeset/fromrecord` | List solution changes from record |

### Platform Analytics
| JSON Endpoint | Description |
|---------------|-------------|
| `POST /analytics/events/summaries` | Get event summaries |
| `GET /analytics/reads` | Get read summaries |

### Audit Logs
| JSON Endpoint | Description |
|---------------|-------------|
| `POST /audit` | Get audit logs |

### Field Usage
| JSON Endpoint | Description |
|---------------|-------------|
| `GET /fields/usage` | Get usage statistics for all fields |
| `GET /fields/usage/{fieldId}` | Get usage statistics for a specific field |

### Document Templates
| JSON Endpoint | Description |
|---------------|-------------|
| `GET /docTemplates/{templateId}/generate` | Generate a document from template |

### Formula Evaluation
| JSON Endpoint | Description |
|---------------|-------------|
| `POST /formula/run` | Run/evaluate a formula |

### App Events
| JSON Endpoint | Description |
|---------------|-------------|
| `GET /apps/{appId}/events` | Get app events (automations, webhooks, etc.) |

### User Management (Partial)
| JSON Endpoint | Description |
|---------------|-------------|
| `POST /users` | Get users with filtering |
| `PUT /users/deny` | Deny users |
| `PUT /users/deny/{shouldDeleteFromGroups}` | Deny and remove from groups |
| `PUT /users/undeny` | Undeny users |

---

## XML-Only Capabilities (Not Available in JSON API)

These features are **only available in the XML API**:

### Roles & Role Assignments

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_GetRoleInfo` | Get all roles defined in an application | **Implemented** |
| `API_UserRoles` | Get all users and their role assignments | **Implemented** |
| `API_GetUserRole` | Get roles for a specific user | **Implemented** |
| `API_AddUserToRole` | Assign a user to a role | **Implemented** |
| `API_RemoveUserFromRole` | Remove a user from a role | **Implemented** |
| `API_ChangeUserRole` | Change a user's role | **Implemented** |

### Group Management (Create, Delete, Query)

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_CreateGroup` | Create a new group | **Implemented** |
| `API_DeleteGroup` | Delete a group | **Implemented** |
| `API_CopyGroup` | Copy a group | Not implemented |
| `API_ChangeGroupInfo` | Update group name/description | Not implemented |
| `API_GetGroupRole` | Get roles assigned to a group | **Implemented** |
| `API_AddGroupToRole` | Assign a group to a role | **Implemented** |
| `API_RemoveGroupFromRole` | Remove group from a role | **Implemented** |
| `API_GetUsersInGroup` | List all users in a group | **Implemented** |
| `API_AddUserToGroup` | Add a user to a group | **Implemented** |
| `API_RemoveUserFromGroup` | Remove a user from a group | **Implemented** |
| `API_GrantedGroups` | Get groups a user belongs to | Not implemented |
| `API_GrantedDBsForGroup` | Get apps a group has access to | Not implemented |

### Application Variables (DBVars)

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_GetDBVar` | Get an application variable value | **Implemented** |
| `API_SetDBVar` | Set an application variable value | **Implemented** |

> Note: JSON API can read/write variables during app create/update, but cannot read individual variables.

### Application Discovery & Metadata

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_GetSchema` | Comprehensive schema (fields, reports, DBVars, child tables) | **Implemented** |
| `API_GrantedDBs` | List all apps user can access (across realms) | **Implemented** |
| `API_FindDBByName` | Find an app by name | **Implemented** |
| `API_GetAncestorInfo` | Get app template/copy lineage | Not implemented |
| `API_GetAppDTMInfo` | Get modification timestamps (fast, no auth) | Not implemented |
| `API_GetDBInfo` | Get table metadata (record count, manager, timestamps) | **Implemented** |
| `API_GetNumRecords` | Get record count for a table | **Implemented** |

### Code Pages

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_GetDBPage` | Get stored code page content | **Implemented** |
| `API_AddReplaceDBPage` | Create or update a code page | **Implemented** |

### Webhooks

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_Webhooks_Create` | Create a webhook | Not implemented |
| `API_Webhooks_Edit` | Edit a webhook | Not implemented |
| `API_Webhooks_Delete` | Delete a webhook | Not implemented |
| `API_Webhooks_Activate` | Activate a webhook | Not implemented |
| `API_Webhooks_Deactivate` | Deactivate a webhook | Not implemented |
| `API_Webhooks_Copy` | Copy a webhook | Not implemented |

### User Provisioning

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_GetUserInfo` | Get user info by email | **Implemented** |
| `API_ProvisionUser` | Create/provision a new user | **Implemented** |
| `API_SendInvitation` | Send invitation email | **Implemented** |
| `API_ChangeManager` | Change app/table manager | **Implemented** |
| `API_ChangeRecordOwner` | Change record owner | **Implemented** |

### Field Choice Management

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_FieldAddChoices` | Add choices to multiple-choice field | **Implemented** |
| `API_FieldRemoveChoices` | Remove choices from field | **Implemented** |
| `API_SetKeyField` | Set key field for table | **Implemented** |

### Record Operations (Unique)

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_DoQueryCount` | Get count without fetching data | **Implemented** |
| `API_GetRecordInfo` | Get record with full field metadata | **Implemented** |
| `API_GetRecordAsHTML` | Get record rendered as HTML | Not implemented |
| `API_CopyMasterDetail` | Copy master record with details | Not implemented |
| `API_ImportFromCSV` | Import data from CSV | Not implemented |
| `API_RunImport` | Execute a saved import | Not implemented |
| `API_GenAddRecordForm` | Generate HTML add record form | Not implemented |
| `API_GenResultsTable` | Generate HTML results table | Not implemented |

### Authentication

| XML Endpoint | Description | SDK Status |
|--------------|-------------|------------|
| `API_Authenticate` | Get ticket from username/password | Used internally |
| `API_SignOut` | Sign out / invalidate session | Not implemented |

---

## Overlapping Capabilities

These operations are available in **both APIs**:

### Apps

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Create app | `POST /apps` | `API_CreateDatabase` |
| Get app | `GET /apps/{appId}` | `API_GetDBInfo` (partial) |
| Update app | `POST /apps/{appId}` | `API_RenameApp` (name only) |
| Delete app | `DELETE /apps/{appId}` | `API_DeleteDatabase` |
| Copy app | `POST /apps/{appId}/copy` | `API_CloneDatabase` |

### Tables

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Create table | `POST /tables` | `API_CreateTable` |
| Get tables | `GET /tables` | `API_GetSchema` (via child tables) |
| Get table | `GET /tables/{tableId}` | `API_GetDBInfo` |
| Update table | `POST /tables/{tableId}` | (via schema) |
| Delete table | `DELETE /tables/{tableId}` | `API_DeleteDatabase` |

### Fields

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Create field | `POST /fields` | `API_AddField` |
| Get fields | `GET /fields` | `API_GetSchema` |
| Get field | `GET /fields/{fieldId}` | `API_GetFieldProperties` |
| Update field | `POST /fields/{fieldId}` | `API_SetFieldProperties` |
| Delete field | `DELETE /fields` | `API_DeleteField` |

### Records

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Query records | `POST /records/query` | `API_DoQuery` |
| Insert/Update | `POST /records` | `API_AddRecord`, `API_EditRecord` |
| Delete records | `DELETE /records` | `API_DeleteRecord`, `API_PurgeRecords` |

### Reports

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Get reports | `GET /reports` | `API_GetSchema` (via queries) |
| Get report | `GET /reports/{reportId}` | (partial via schema) |
| Run report | `POST /reports/{reportId}/run` | (via DoQuery with qid) |

### Files

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Download file | `GET /files/{tableId}/{recordId}/{fieldId}/{version}` | (via record data) |
| Delete file | `DELETE /files/{tableId}/{recordId}/{fieldId}/{version}` | (via record edit) |
| Upload file | (via record upsert with base64) | `API_UploadFile` |

### Group Membership (Partial Overlap)

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Add members | `POST /groups/{gid}/members` | `API_AddUserToGroup` |
| Remove members | `DELETE /groups/{gid}/members` | `API_RemoveUserFromGroup` |
| Add subgroups | `POST /groups/{gid}/subgroups` | `API_AddSubGroup` |
| Remove subgroups | `DELETE /groups/{gid}/subgroups` | `API_RemoveSubgroup` |
| Add managers | `POST /groups/{gid}/managers` | (not available) |
| Remove managers | `DELETE /groups/{gid}/managers` | (not available) |

### User Tokens

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Clone token | `POST /usertoken/clone` | (not available) |
| Transfer token | `POST /usertoken/transfer` | (not available) |
| Deactivate token | `POST /usertoken/deactivate` | (not available) |
| Delete token | `DELETE /usertoken` | (not available) |

### Authentication

| Operation | JSON API | XML API |
|-----------|----------|---------|
| Get temp token | `GET /auth/temporary/{dbid}` | (not available) |
| Exchange SSO token | `POST /auth/oauth/token` | (not available) |
| Username/password | (not available) | `API_Authenticate` |

---

## Implementation Priority

### Currently Implemented

**App Discovery:**
- `API_GrantedDBs` - `xml.GrantedDBs()`
- `API_FindDBByName` - `xml.FindDBByName()`
- `API_GetDBInfo` - `xml.GetDBInfo()`
- `API_GetNumRecords` - `xml.GetNumRecords()`

**Role Management:**
- `API_GetRoleInfo` - `xml.GetRoleInfo()`
- `API_UserRoles` - `xml.UserRoles()`
- `API_GetUserRole` - `xml.GetUserRole()`
- `API_AddUserToRole` - `xml.AddUserToRole()`
- `API_RemoveUserFromRole` - `xml.RemoveUserFromRole()`
- `API_ChangeUserRole` - `xml.ChangeUserRole()`

**Group Management:**
- `API_CreateGroup` - `xml.CreateGroup()`
- `API_DeleteGroup` - `xml.DeleteGroup()`
- `API_GetUsersInGroup` - `xml.GetUsersInGroup()`
- `API_AddUserToGroup` - `xml.AddUserToGroup()`
- `API_RemoveUserFromGroup` - `xml.RemoveUserFromGroup()`
- `API_GetGroupRole` - `xml.GetGroupRole()`
- `API_AddGroupToRole` - `xml.AddGroupToRole()`
- `API_RemoveGroupFromRole` - `xml.RemoveGroupFromRole()`

**User Management:**
- `API_GetUserInfo` - `xml.GetUserInfo()`
- `API_ProvisionUser` - `xml.ProvisionUser()`
- `API_SendInvitation` - `xml.SendInvitation()`
- `API_ChangeManager` - `xml.ChangeManager()`
- `API_ChangeRecordOwner` - `xml.ChangeRecordOwner()`

**Application Variables:**
- `API_GetDBVar` - `xml.GetDBVar()`
- `API_SetDBVar` - `xml.SetDBVar()`

**Code Pages:**
- `API_GetDBPage` - `xml.GetDBPage()`
- `API_AddReplaceDBPage` - `xml.AddReplaceDBPage()`

**Field Management:**
- `API_FieldAddChoices` - `xml.FieldAddChoices()`
- `API_FieldRemoveChoices` - `xml.FieldRemoveChoices()`
- `API_SetKeyField` - `xml.SetKeyField()`

**Schema Information:**
- `API_GetSchema` - `xml.GetSchema()`

**Record Information:**
- `API_DoQueryCount` - `xml.DoQueryCount()`
- `API_GetRecordInfo` - `xml.GetRecordInfo()`
- `API_GetRecordInfoByKey` - `xml.GetRecordInfoByKey()`

### Not Yet Implemented (Niche Use Cases)

1. `API_GetAppDTMInfo` - Fast change detection (no auth required)
2. `API_GetAncestorInfo` - Template lineage
3. `API_CopyGroup` / `API_ChangeGroupInfo` - Group utilities
4. `API_GrantedGroups` / `API_GrantedDBsForGroup` - Group discovery
5. Webhook management (`API_Webhooks_*`)
6. CSV import (`API_ImportFromCSV`, `API_RunImport`)
7. HTML generation (`API_GenAddRecordForm`, `API_GenResultsTable`, `API_GetRecordAsHTML`)
8. `API_CopyMasterDetail` - Copy master record with details

---

## Usage Example

```go
import (
    "github.com/DrewBradfordXYZ/quickbase-go"
    "github.com/DrewBradfordXYZ/quickbase-go/xml"
)

// Create main client (JSON API)
qb, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))

// Create XML client for legacy endpoints
xmlClient := xml.New(qb)

// Get all roles (XML-only)
roles, _ := xmlClient.GetRoleInfo(ctx, appId)
for _, role := range roles.Roles {
    fmt.Printf("Role %d: %s\n", role.ID, role.Name)
}

// Get relationships (JSON-only)
rels, _ := qb.GetRelationships(tableId).Run(ctx)
for _, rel := range rels {
    fmt.Printf("Relationship: %s -> %s\n", rel.ParentTableId, rel.ChildTableId)
}
```

---

## References

- [QuickBase XML API Call Reference](https://help.quickbase.com/docs/quickbase-api-call-reference)
- [QuickBase JSON RESTful API](https://developer.quickbase.com/)
- [API Call Reference by Function](https://help.quickbase.com/docs/api-call-reference-by-function)
