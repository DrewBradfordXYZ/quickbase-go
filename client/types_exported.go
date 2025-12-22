// Package client provides a QuickBase API client.
//
// This file re-exports types from internal/generated to make them available
// for consumers who need direct access to the Raw* API methods.
package client

import "github.com/DrewBradfordXYZ/quickbase-go/internal/generated"

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
// Nested Types (for accessing response data)
// =============================================================================

// App types
type GetApp_200_SecurityProperties = generated.GetApp_200_SecurityProperties
type GetApp_200_Variables_Item = generated.GetApp_200_Variables_Item

// App events types
type GetAppEvents200Type = generated.GetAppEvents200Type
type GetAppEvents_200_Owner = generated.GetAppEvents_200_Owner
type GetAppEvents_200_Item = generated.GetAppEvents_200_Item

// Tables types
type GetAppTables_200_Item = generated.GetAppTables_200_Item

// Fields types
type GetFields_200_Properties_CompositeFields_Item = generated.GetFields_200_Properties_CompositeFields_Item
type GetFields_200_Properties = generated.GetFields_200_Properties
type GetFields_200_Item = generated.GetFields_200_Item
type GetField_200_Properties_CompositeFields_Item = generated.GetField_200_Properties_CompositeFields_Item
type GetField_200_Properties = generated.GetField_200_Properties

// Field usage types (GetFieldsUsage)
type GetFieldsUsage_200_Field = generated.GetFieldsUsage_200_Field
type GetFieldsUsage_200_Usage_Actions = generated.GetFieldsUsage_200_Usage_Actions
type GetFieldsUsage_200_Usage_AppHomePages = generated.GetFieldsUsage_200_Usage_AppHomePages
type GetFieldsUsage_200_Usage_DefaultReports = generated.GetFieldsUsage_200_Usage_DefaultReports
type GetFieldsUsage_200_Usage_ExactForms = generated.GetFieldsUsage_200_Usage_ExactForms
type GetFieldsUsage_200_Usage_Fields = generated.GetFieldsUsage_200_Usage_Fields
type GetFieldsUsage_200_Usage_Forms = generated.GetFieldsUsage_200_Usage_Forms
type GetFieldsUsage_200_Usage_Notifications = generated.GetFieldsUsage_200_Usage_Notifications
type GetFieldsUsage_200_Usage_PersonalReports = generated.GetFieldsUsage_200_Usage_PersonalReports
type GetFieldsUsage_200_Usage_Pipelines = generated.GetFieldsUsage_200_Usage_Pipelines
type GetFieldsUsage_200_Usage_Relationships = generated.GetFieldsUsage_200_Usage_Relationships
type GetFieldsUsage_200_Usage_Reminders = generated.GetFieldsUsage_200_Usage_Reminders
type GetFieldsUsage_200_Usage_Reports = generated.GetFieldsUsage_200_Usage_Reports
type GetFieldsUsage_200_Usage_Roles = generated.GetFieldsUsage_200_Usage_Roles
type GetFieldsUsage_200_Usage_Webhooks = generated.GetFieldsUsage_200_Usage_Webhooks
type GetFieldsUsage_200_Usage = generated.GetFieldsUsage_200_Usage
type GetFieldsUsage_200_Item = generated.GetFieldsUsage_200_Item

// Field usage types (GetFieldUsage - single field)
type GetFieldUsage_200_Field = generated.GetFieldUsage_200_Field
type GetFieldUsage_200_Usage_Actions = generated.GetFieldUsage_200_Usage_Actions
type GetFieldUsage_200_Usage_AppHomePages = generated.GetFieldUsage_200_Usage_AppHomePages
type GetFieldUsage_200_Usage_DefaultReports = generated.GetFieldUsage_200_Usage_DefaultReports
type GetFieldUsage_200_Usage_ExactForms = generated.GetFieldUsage_200_Usage_ExactForms
type GetFieldUsage_200_Usage_Fields = generated.GetFieldUsage_200_Usage_Fields
type GetFieldUsage_200_Usage_Forms = generated.GetFieldUsage_200_Usage_Forms
type GetFieldUsage_200_Usage_Notifications = generated.GetFieldUsage_200_Usage_Notifications
type GetFieldUsage_200_Usage_PersonalReports = generated.GetFieldUsage_200_Usage_PersonalReports
type GetFieldUsage_200_Usage_Pipelines = generated.GetFieldUsage_200_Usage_Pipelines
type GetFieldUsage_200_Usage_Relationships = generated.GetFieldUsage_200_Usage_Relationships
type GetFieldUsage_200_Usage_Reminders = generated.GetFieldUsage_200_Usage_Reminders
type GetFieldUsage_200_Usage_Reports = generated.GetFieldUsage_200_Usage_Reports
type GetFieldUsage_200_Usage_Roles = generated.GetFieldUsage_200_Usage_Roles
type GetFieldUsage_200_Usage_Webhooks = generated.GetFieldUsage_200_Usage_Webhooks
type GetFieldUsage_200_Usage = generated.GetFieldUsage_200_Usage
type GetFieldUsage_200_Item = generated.GetFieldUsage_200_Item

// Relationships types
type GetRelationships_200_Metadata = generated.GetRelationships_200_Metadata
type GetRelationships_200_Relationships_ForeignKeyField = generated.GetRelationships_200_Relationships_ForeignKeyField
type GetRelationships_200_Relationships_LookupFields_Item = generated.GetRelationships_200_Relationships_LookupFields_Item
type GetRelationships_200_Relationships_SummaryFields_Item = generated.GetRelationships_200_Relationships_SummaryFields_Item
type GetRelationships_200_Relationships_Item = generated.GetRelationships_200_Relationships_Item

// Reports types
type GetTableReports_200_Query_FormulaFields_Item = generated.GetTableReports_200_Query_FormulaFields_Item
type GetTableReports_200_Query = generated.GetTableReports_200_Query
type GetTableReports_200_Item = generated.GetTableReports_200_Item
type GetReport_200_Query_FormulaFields_Item = generated.GetReport_200_Query_FormulaFields_Item
type GetReport_200_Query = generated.GetReport_200_Query

