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
// Named Response Data Types
// =============================================================================
// NOTE: Data and Item types are now generated with wrapper types in
// results_generated.go, which provide nil-safe accessor methods.
// Use the wrapper types (e.g., AppResult, FieldResult, TableReportsItem)
// instead of raw generated types for better ergonomics.
