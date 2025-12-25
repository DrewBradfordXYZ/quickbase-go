// Package client provides a QuickBase API client.
//
// This file re-exports types from generated to make them available
// for consumers who need direct access to the Raw* API methods.
package client

import "github.com/DrewBradfordXYZ/quickbase-go/generated"

// =============================================================================
// Parameter Types (for Raw* method calls)
// =============================================================================

type ChangesetSolutionFromRecordParams = generated.ChangesetSolutionFromRecordParams
type ChangesetSolutionParams = generated.ChangesetSolutionParams
type CreateFieldParams = generated.CreateFieldParams
type CreateSolutionFromRecordParams = generated.CreateSolutionFromRecordParams
type CreateSolutionParams = generated.CreateSolutionParams
type CreateTableParams = generated.CreateTableParams
type DeleteFieldsParams = generated.DeleteFieldsParams
type DeleteTableParams = generated.DeleteTableParams
type DenyUsersAndGroupsParams = generated.DenyUsersAndGroupsParams
type DenyUsersParams = generated.DenyUsersParams
type ExportSolutionParams = generated.ExportSolutionParams
type ExportSolutionToRecordParams = generated.ExportSolutionToRecordParams
type GenerateDocumentParams = generated.GenerateDocumentParams
type GetAppTablesParams = generated.GetAppTablesParams
type GetFieldsParams = generated.GetFieldsParams
type GetFieldParams = generated.GetFieldParams
type GetFieldsUsageParams = generated.GetFieldsUsageParams
type GetFieldUsageParams = generated.GetFieldUsageParams
type GetRelationshipsParams = generated.GetRelationshipsParams
type GetReportParams = generated.GetReportParams
type GetTableParams = generated.GetTableParams
type GetTableReportsParams = generated.GetTableReportsParams
type GetTempTokenDBIDParams = generated.GetTempTokenDBIDParams
type GetUsersParams = generated.GetUsersParams
type PlatformAnalyticEventSummariesParams = generated.PlatformAnalyticEventSummariesParams
type PlatformAnalyticReadsParams = generated.PlatformAnalyticReadsParams
type RunReportParams = generated.RunReportParams
type UndenyUsersParams = generated.UndenyUsersParams
type UpdateFieldParams = generated.UpdateFieldParams
type UpdateSolutionParams = generated.UpdateSolutionParams
type UpdateSolutionToRecordParams = generated.UpdateSolutionToRecordParams
type UpdateTableParams = generated.UpdateTableParams

// =============================================================================
// Request Body Types (for Raw* method calls)
// =============================================================================

type AddManagersToGroupJSONRequestBody = generated.AddManagersToGroupJSONRequestBody
type AddMembersToGroupJSONRequestBody = generated.AddMembersToGroupJSONRequestBody
type AddSubgroupsToGroupJSONRequestBody = generated.AddSubgroupsToGroupJSONRequestBody
type AuditJSONRequestBody = generated.AuditJSONRequestBody
type ChangesetSolutionJSONRequestBody = generated.ChangesetSolutionJSONRequestBody
type CloneUserTokenJSONRequestBody = generated.CloneUserTokenJSONRequestBody
type CopyAppJSONRequestBody = generated.CopyAppJSONRequestBody
type CreateAppJSONRequestBody = generated.CreateAppJSONRequestBody
type CreateFieldJSONRequestBody = generated.CreateFieldJSONRequestBody
type CreateRelationshipJSONRequestBody = generated.CreateRelationshipJSONRequestBody
type CreateSolutionJSONRequestBody = generated.CreateSolutionJSONRequestBody
type CreateTableJSONRequestBody = generated.CreateTableJSONRequestBody
type DeleteAppJSONRequestBody = generated.DeleteAppJSONRequestBody
type DeleteFieldsJSONRequestBody = generated.DeleteFieldsJSONRequestBody
type DenyUsersAndGroupsJSONRequestBody = generated.DenyUsersAndGroupsJSONRequestBody
type DenyUsersJSONRequestBody = generated.DenyUsersJSONRequestBody
type ExchangeSsoTokenJSONRequestBody = generated.ExchangeSsoTokenJSONRequestBody
type GetUsersJSONRequestBody = generated.GetUsersJSONRequestBody
type PlatformAnalyticEventSummariesJSONRequestBody = generated.PlatformAnalyticEventSummariesJSONRequestBody
type RemoveManagersFromGroupJSONRequestBody = generated.RemoveManagersFromGroupJSONRequestBody
type RemoveMembersFromGroupJSONRequestBody = generated.RemoveMembersFromGroupJSONRequestBody
type RemoveSubgroupsFromGroupJSONRequestBody = generated.RemoveSubgroupsFromGroupJSONRequestBody
type RunFormulaJSONRequestBody = generated.RunFormulaJSONRequestBody
type RunReportJSONRequestBody = generated.RunReportJSONRequestBody
type TransferUserTokenJSONRequestBody = generated.TransferUserTokenJSONRequestBody
type UndenyUsersJSONRequestBody = generated.UndenyUsersJSONRequestBody
type UpdateAppJSONRequestBody = generated.UpdateAppJSONRequestBody
type UpdateFieldJSONRequestBody = generated.UpdateFieldJSONRequestBody
type UpdateRelationshipJSONRequestBody = generated.UpdateRelationshipJSONRequestBody
type UpdateSolutionJSONRequestBody = generated.UpdateSolutionJSONRequestBody
type UpdateTableJSONRequestBody = generated.UpdateTableJSONRequestBody

// =============================================================================
// Response Types (returned by Raw* methods)
// =============================================================================

type AddManagersToGroupResponse = generated.AddManagersToGroupResponse
type AddMembersToGroupResponse = generated.AddMembersToGroupResponse
type AddSubgroupsToGroupResponse = generated.AddSubgroupsToGroupResponse
type AuditResponse = generated.AuditResponse
type ChangesetSolutionFromRecordResponse = generated.ChangesetSolutionFromRecordResponse
type ChangesetSolutionResponse = generated.ChangesetSolutionResponse
type CloneUserTokenResponse = generated.CloneUserTokenResponse
type CopyAppResponse = generated.CopyAppResponse
type CreateAppResponse = generated.CreateAppResponse
type CreateFieldResponse = generated.CreateFieldResponse
type CreateRelationshipResponse = generated.CreateRelationshipResponse
type CreateSolutionFromRecordResponse = generated.CreateSolutionFromRecordResponse
type CreateSolutionResponse = generated.CreateSolutionResponse
type CreateTableResponse = generated.CreateTableResponse
type DeactivateUserTokenResponse = generated.DeactivateUserTokenResponse
type DeleteAppResponse = generated.DeleteAppResponse
type DeleteFieldsResponse = generated.DeleteFieldsResponse
type DeleteFileResponse = generated.DeleteFileResponse
type DeleteRelationshipResponse = generated.DeleteRelationshipResponse
type DeleteTableResponse = generated.DeleteTableResponse
type DeleteUserTokenResponse = generated.DeleteUserTokenResponse
type DenyUsersAndGroupsResponse = generated.DenyUsersAndGroupsResponse
type DenyUsersResponse = generated.DenyUsersResponse
type DownloadFileResponse = generated.DownloadFileResponse
type ExchangeSsoTokenResponse = generated.ExchangeSsoTokenResponse
type ExportSolutionResponse = generated.ExportSolutionResponse
type ExportSolutionToRecordResponse = generated.ExportSolutionToRecordResponse
type GenerateDocumentResponse = generated.GenerateDocumentResponse
type GetAppEventsResponse = generated.GetAppEventsResponse
type GetAppTablesResponse = generated.GetAppTablesResponse
type GetFieldResponse = generated.GetFieldResponse
type GetFieldsUsageResponse = generated.GetFieldsUsageResponse
type GetFieldUsageResponse = generated.GetFieldUsageResponse
type GetRelationshipsResponse = generated.GetRelationshipsResponse
type GetReportResponse = generated.GetReportResponse
type GetTableReportsResponse = generated.GetTableReportsResponse
type GetTableResponse = generated.GetTableResponse
type GetTempTokenDBIDResponse = generated.GetTempTokenDBIDResponse
type GetUsersResponse = generated.GetUsersResponse
type PlatformAnalyticEventSummariesResponse = generated.PlatformAnalyticEventSummariesResponse
type PlatformAnalyticReadsResponse = generated.PlatformAnalyticReadsResponse
type RemoveManagersFromGroupResponse = generated.RemoveManagersFromGroupResponse
type RemoveMembersFromGroupResponse = generated.RemoveMembersFromGroupResponse
type RemoveSubgroupsFromGroupResponse = generated.RemoveSubgroupsFromGroupResponse
type RunFormulaResponse = generated.RunFormulaResponse
type RunReportResponse = generated.RunReportResponse
type TransferUserTokenResponse = generated.TransferUserTokenResponse
type UndenyUsersResponse = generated.UndenyUsersResponse
type UpdateAppResponse = generated.UpdateAppResponse
type UpdateFieldResponse = generated.UpdateFieldResponse
type UpdateRelationshipResponse = generated.UpdateRelationshipResponse
type UpdateSolutionResponse = generated.UpdateSolutionResponse
type UpdateSolutionToRecordResponse = generated.UpdateSolutionToRecordResponse
type UpdateTableResponse = generated.UpdateTableResponse

// =============================================================================
// Named Response Data Types (extracted from inline schemas)
// =============================================================================

// App types
type GetAppData = generated.GetAppData
type GetAppData_SecurityProperties = generated.GetAppData_SecurityProperties
type GetAppData_Variables_Item = generated.GetAppData_Variables_Item
type CreateAppData = generated.CreateAppData
type UpdateAppData = generated.UpdateAppData
type DeleteAppData = generated.DeleteAppData
type CopyAppData = generated.CopyAppData

// App events types
type GetAppEventsItem = generated.GetAppEventsItem
type GetAppEventsItemType = generated.GetAppEventsItemType
type GetAppEventsItem_Owner = generated.GetAppEventsItem_Owner

// Tables types
type GetAppTablesItem = generated.GetAppTablesItem
type GetTableData = generated.GetTableData
type CreateTableData = generated.CreateTableData
type UpdateTableData = generated.UpdateTableData
type DeleteTableData = generated.DeleteTableData

// Roles types
type GetRolesItem = generated.GetRolesItem

// Fields types
type GetFieldsItem = generated.GetFieldsItem
type GetFieldsItem_Properties = generated.GetFieldsItem_Properties
type GetFieldsItem_Properties_CompositeFields_Item = generated.GetFieldsItem_Properties_CompositeFields_Item
type GetFieldData = generated.GetFieldData
type GetFieldData_Properties = generated.GetFieldData_Properties
type GetFieldData_Properties_CompositeFields_Item = generated.GetFieldData_Properties_CompositeFields_Item
type CreateFieldData = generated.CreateFieldData
type UpdateFieldData = generated.UpdateFieldData
type DeleteFieldsData = generated.DeleteFieldsData

// Field usage types
type GetFieldsUsageItem = generated.GetFieldsUsageItem
type GetFieldsUsageItem_Field = generated.GetFieldsUsageItem_Field
type GetFieldsUsageItem_Usage = generated.GetFieldsUsageItem_Usage
type GetFieldsUsageItem_Usage_Actions = generated.GetFieldsUsageItem_Usage_Actions
type GetFieldsUsageItem_Usage_AppHomePages = generated.GetFieldsUsageItem_Usage_AppHomePages
type GetFieldsUsageItem_Usage_DefaultReports = generated.GetFieldsUsageItem_Usage_DefaultReports
type GetFieldsUsageItem_Usage_ExactForms = generated.GetFieldsUsageItem_Usage_ExactForms
type GetFieldsUsageItem_Usage_Fields = generated.GetFieldsUsageItem_Usage_Fields
type GetFieldsUsageItem_Usage_Forms = generated.GetFieldsUsageItem_Usage_Forms
type GetFieldsUsageItem_Usage_Notifications = generated.GetFieldsUsageItem_Usage_Notifications
type GetFieldsUsageItem_Usage_PersonalReports = generated.GetFieldsUsageItem_Usage_PersonalReports
type GetFieldsUsageItem_Usage_Pipelines = generated.GetFieldsUsageItem_Usage_Pipelines
type GetFieldsUsageItem_Usage_Relationships = generated.GetFieldsUsageItem_Usage_Relationships
type GetFieldsUsageItem_Usage_Reminders = generated.GetFieldsUsageItem_Usage_Reminders
type GetFieldsUsageItem_Usage_Reports = generated.GetFieldsUsageItem_Usage_Reports
type GetFieldsUsageItem_Usage_Roles = generated.GetFieldsUsageItem_Usage_Roles
type GetFieldsUsageItem_Usage_Webhooks = generated.GetFieldsUsageItem_Usage_Webhooks

type GetFieldUsageItem = generated.GetFieldUsageItem
type GetFieldUsageItem_Field = generated.GetFieldUsageItem_Field
type GetFieldUsageItem_Usage = generated.GetFieldUsageItem_Usage
type GetFieldUsageItem_Usage_Actions = generated.GetFieldUsageItem_Usage_Actions
type GetFieldUsageItem_Usage_AppHomePages = generated.GetFieldUsageItem_Usage_AppHomePages
type GetFieldUsageItem_Usage_DefaultReports = generated.GetFieldUsageItem_Usage_DefaultReports
type GetFieldUsageItem_Usage_ExactForms = generated.GetFieldUsageItem_Usage_ExactForms
type GetFieldUsageItem_Usage_Fields = generated.GetFieldUsageItem_Usage_Fields
type GetFieldUsageItem_Usage_Forms = generated.GetFieldUsageItem_Usage_Forms
type GetFieldUsageItem_Usage_Notifications = generated.GetFieldUsageItem_Usage_Notifications
type GetFieldUsageItem_Usage_PersonalReports = generated.GetFieldUsageItem_Usage_PersonalReports
type GetFieldUsageItem_Usage_Pipelines = generated.GetFieldUsageItem_Usage_Pipelines
type GetFieldUsageItem_Usage_Relationships = generated.GetFieldUsageItem_Usage_Relationships
type GetFieldUsageItem_Usage_Reminders = generated.GetFieldUsageItem_Usage_Reminders
type GetFieldUsageItem_Usage_Reports = generated.GetFieldUsageItem_Usage_Reports
type GetFieldUsageItem_Usage_Roles = generated.GetFieldUsageItem_Usage_Roles
type GetFieldUsageItem_Usage_Webhooks = generated.GetFieldUsageItem_Usage_Webhooks

// Relationships types
type GetRelationshipsData = generated.GetRelationshipsData
type GetRelationshipsData_Metadata = generated.GetRelationshipsData_Metadata
type GetRelationshipsData_Relationships_Item = generated.GetRelationshipsData_Relationships_Item
type GetRelationshipsData_Relationships_ForeignKeyField = generated.GetRelationshipsData_Relationships_ForeignKeyField
type GetRelationshipsData_Relationships_LookupFields_Item = generated.GetRelationshipsData_Relationships_LookupFields_Item
type GetRelationshipsData_Relationships_SummaryFields_Item = generated.GetRelationshipsData_Relationships_SummaryFields_Item
type CreateRelationshipData = generated.CreateRelationshipData
type UpdateRelationshipData = generated.UpdateRelationshipData
type DeleteRelationshipData = generated.DeleteRelationshipData

// Reports types
type GetTableReportsItem = generated.GetTableReportsItem
type GetTableReportsItem_Query = generated.GetTableReportsItem_Query
type GetTableReportsItem_Query_FormulaFields_Item = generated.GetTableReportsItem_Query_FormulaFields_Item
type GetReportData = generated.GetReportData
type GetReportData_Query = generated.GetReportData_Query
type GetReportData_Query_FormulaFields_Item = generated.GetReportData_Query_FormulaFields_Item
type RunReportData = generated.RunReportData

// Query types
type RunQueryData = generated.RunQueryData
type DeleteRecordsData = generated.DeleteRecordsData
type UpsertData = generated.UpsertData

// Users types
type GetUsersData = generated.GetUsersData
type DenyUsersData = generated.DenyUsersData
type UndenyUsersData = generated.UndenyUsersData
type DenyUsersAndGroupsData = generated.DenyUsersAndGroupsData

// Group types
type AddMembersToGroupData = generated.AddMembersToGroupData
type RemoveMembersFromGroupData = generated.RemoveMembersFromGroupData
type AddManagersToGroupData = generated.AddManagersToGroupData
type RemoveManagersFromGroupData = generated.RemoveManagersFromGroupData
type AddSubgroupsToGroupData = generated.AddSubgroupsToGroupData
type RemoveSubgroupsFromGroupData = generated.RemoveSubgroupsFromGroupData

// Auth types
type GetTempTokenDBIDData = generated.GetTempTokenDBIDData
type ExchangeSsoTokenData = generated.ExchangeSsoTokenData
type CloneUserTokenData = generated.CloneUserTokenData
type TransferUserTokenData = generated.TransferUserTokenData
type DeactivateUserTokenData = generated.DeactivateUserTokenData
type DeleteUserTokenData = generated.DeleteUserTokenData

// File types
type DeleteFileData = generated.DeleteFileData

// Formula types
type RunFormulaData = generated.RunFormulaData

// Audit types
type AuditData = generated.AuditData

// Analytics types
type PlatformAnalyticReadsData = generated.PlatformAnalyticReadsData
type PlatformAnalyticEventSummariesData = generated.PlatformAnalyticEventSummariesData

// Trustees types
type GetTrusteesItem = generated.GetTrusteesItem
type AddTrusteesData = generated.AddTrusteesData
type RemoveTrusteesData = generated.RemoveTrusteesData
type UpdateTrusteesData = generated.UpdateTrusteesData

// Document types
type GenerateDocumentData = generated.GenerateDocumentData
