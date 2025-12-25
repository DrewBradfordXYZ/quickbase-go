package xml

import (
	"context"
	"errors"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v2/core"
)

// mockCaller implements the Caller interface for testing.
type mockCaller struct {
	realm    string
	response []byte
	err      error

	// Captured request details for assertions
	lastDBID   string
	lastAction string
	lastBody   []byte
}

func (m *mockCaller) Realm() string {
	return m.realm
}

func (m *mockCaller) DoXML(ctx context.Context, dbid, action string, body []byte) ([]byte, error) {
	m.lastDBID = dbid
	m.lastAction = action
	m.lastBody = body
	return m.response, m.err
}

func TestGetRoleInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetRoleInfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <roles>
      <role id="10">
         <name>Viewer</name>
         <access id="3">Basic Access</access>
      </role>
      <role id="11">
         <name>Participant</name>
         <access id="3">Basic Access</access>
      </role>
      <role id="12">
         <name>Administrator</name>
         <access id="1">Administrator</access>
      </role>
   </roles>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetRoleInfo(context.Background(), "bqxyz123")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetRoleInfo" {
			t.Errorf("expected action API_GetRoleInfo, got %s", mock.lastAction)
		}

		if mock.lastDBID != "bqxyz123" {
			t.Errorf("expected dbid bqxyz123, got %s", mock.lastDBID)
		}

		if len(result.Roles) != 3 {
			t.Fatalf("expected 3 roles, got %d", len(result.Roles))
		}

		// Check first role
		if result.Roles[0].ID != 10 {
			t.Errorf("expected role ID 10, got %d", result.Roles[0].ID)
		}
		if result.Roles[0].Name != "Viewer" {
			t.Errorf("expected role name Viewer, got %s", result.Roles[0].Name)
		}
		if result.Roles[0].Access.ID != 3 {
			t.Errorf("expected access ID 3, got %d", result.Roles[0].Access.ID)
		}

		// Check admin role
		if result.Roles[2].Name != "Administrator" {
			t.Errorf("expected role name Administrator, got %s", result.Roles[2].Name)
		}
		if result.Roles[2].Access.ID != 1 {
			t.Errorf("expected access ID 1, got %d", result.Roles[2].Access.ID)
		}
	})

	t.Run("error response", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetRoleInfo</action>
   <errcode>4</errcode>
   <errtext>User not authorized</errtext>
</qdbapi>`),
		}

		client := New(mock)
		_, err := client.GetRoleInfo(context.Background(), "bqxyz123")

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var xmlErr *Error
		if !errors.As(err, &xmlErr) {
			t.Fatalf("expected *Error, got %T", err)
		}

		if xmlErr.Code != 4 {
			t.Errorf("expected error code 4, got %d", xmlErr.Code)
		}

		if !IsUnauthorized(err) {
			t.Error("expected IsUnauthorized to return true")
		}
	})

	t.Run("network error", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			err:   errors.New("connection refused"),
		}

		client := New(mock)
		_, err := client.GetRoleInfo(context.Background(), "bqxyz123")

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestUserRoles(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_UserRoles</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <users>
      <user type="user" id="112149.bhsv">
         <name>Jack Danielsson</name>
         <lastAccess>1403035235243</lastAccess>
         <lastAccessAppLocal>06-17-2014 01:00 PM</lastAccessAppLocal>
         <firstName>Jack</firstName>
         <lastName>Danielsson</lastName>
         <roles>
            <role id="12">
               <name>Administrator</name>
               <access id="1">Administrator</access>
            </role>
         </roles>
      </user>
      <user type="group" id="3">
        <name>Everyone on the Internet</name>
        <roles>
         <role id="10">
            <name>Viewer</name>
            <access id="3">Basic Access</access>
          </role>
        </roles>
      </user>
   </users>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.UserRoles(context.Background(), "bqxyz123")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_UserRoles" {
			t.Errorf("expected action API_UserRoles, got %s", mock.lastAction)
		}

		if len(result.Users) != 2 {
			t.Fatalf("expected 2 users, got %d", len(result.Users))
		}

		// Check individual user
		user := result.Users[0]
		if user.ID != "112149.bhsv" {
			t.Errorf("expected user ID 112149.bhsv, got %s", user.ID)
		}
		if user.Type != "user" {
			t.Errorf("expected type user, got %s", user.Type)
		}
		if user.Name != "Jack Danielsson" {
			t.Errorf("expected name Jack Danielsson, got %s", user.Name)
		}
		if user.FirstName != "Jack" {
			t.Errorf("expected firstName Jack, got %s", user.FirstName)
		}
		if len(user.Roles) != 1 {
			t.Fatalf("expected 1 role, got %d", len(user.Roles))
		}
		if user.Roles[0].Name != "Administrator" {
			t.Errorf("expected role Administrator, got %s", user.Roles[0].Name)
		}

		// Check group
		group := result.Users[1]
		if group.Type != "group" {
			t.Errorf("expected type group, got %s", group.Type)
		}
		if group.Name != "Everyone on the Internet" {
			t.Errorf("expected name Everyone on the Internet, got %s", group.Name)
		}
	})
}

func TestGetUserRole(t *testing.T) {
	t.Run("success with groups", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetUserRole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <user id="112245.efy7">
   <name>John Doe</name>
      <roles>
         <role id="11">
           <name>Participant</name>
           <access id="3">Basic Access</access>
           <member type="user">John Doe</member>
         </role>
         <role id="10">
           <name>Viewer</name>
           <access id="3">Basic Access</access>
           <member type="group">Group1</member>
         </role>
      </roles>
   </user>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetUserRole(context.Background(), "bqxyz123", "112245.efy7", true)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetUserRole" {
			t.Errorf("expected action API_GetUserRole, got %s", mock.lastAction)
		}

		// Verify request body contains userid and inclgrps
		body := string(mock.lastBody)
		if body != "<qdbapi><userid>112245.efy7</userid><inclgrps>1</inclgrps></qdbapi>" {
			t.Errorf("unexpected request body: %s", body)
		}

		if result.UserID != "112245.efy7" {
			t.Errorf("expected user ID 112245.efy7, got %s", result.UserID)
		}
		if result.UserName != "John Doe" {
			t.Errorf("expected user name John Doe, got %s", result.UserName)
		}

		if len(result.Roles) != 2 {
			t.Fatalf("expected 2 roles, got %d", len(result.Roles))
		}

		// Check direct role
		if result.Roles[0].Member == nil {
			t.Fatal("expected member to be set")
		}
		if result.Roles[0].Member.Type != "user" {
			t.Errorf("expected member type user, got %s", result.Roles[0].Member.Type)
		}

		// Check group role
		if result.Roles[1].Member.Type != "group" {
			t.Errorf("expected member type group, got %s", result.Roles[1].Member.Type)
		}
		if result.Roles[1].Member.Name != "Group1" {
			t.Errorf("expected member name Group1, got %s", result.Roles[1].Member.Name)
		}
	})

	t.Run("without groups", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetUserRole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <user id="112245.efy7">
   <name>John Doe</name>
      <roles>
         <role id="11">
           <name>Participant</name>
           <access id="3">Basic Access</access>
         </role>
      </roles>
   </user>
</qdbapi>`),
		}

		client := New(mock)
		_, err := client.GetUserRole(context.Background(), "bqxyz123", "112245.efy7", false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify request body does NOT contain inclgrps
		body := string(mock.lastBody)
		if body != "<qdbapi><userid>112245.efy7</userid></qdbapi>" {
			t.Errorf("unexpected request body: %s", body)
		}
	})
}

func TestGetSchema(t *testing.T) {
	t.Run("app level schema", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
     <action>API_GetSchema</action>
     <errcode>0</errcode>
     <errtext>No error</errtext>
     <time_zone>(UTC-08:00) Pacific Time (US &amp; Canada)</time_zone>
     <date_format>YYYY-MM-DD</date_format>
     <table>
          <name>API created Sample</name>
          <desc>This is a sample application.</desc>
          <original>
                <app_id>bdb5rjd6h</app_id>
                <table_id>bdb5rjd6h</table_id>
                <cre_date>1204586581894</cre_date>
                <mod_date>1206394201119</mod_date>
                <next_record_id>1</next_record_id>
                <next_field_id>7</next_field_id>
          </original>
          <variables>
               <var name="Blue">14</var>
               <var name="Jack">14</var>
          </variables>
          <chdbids>
              <chdbid name="_dbid_vehicle">bddrydqhg</chdbid>
          </chdbids>
      </table>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetSchema(context.Background(), "bdb5rjd6h")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetSchema" {
			t.Errorf("expected action API_GetSchema, got %s", mock.lastAction)
		}

		if result.TimeZone != "(UTC-08:00) Pacific Time (US & Canada)" {
			t.Errorf("unexpected timezone: %s", result.TimeZone)
		}
		if result.DateFormat != "YYYY-MM-DD" {
			t.Errorf("unexpected date format: %s", result.DateFormat)
		}

		if result.Table.Name != "API created Sample" {
			t.Errorf("expected table name 'API created Sample', got %s", result.Table.Name)
		}

		if len(result.Table.Variables) != 2 {
			t.Fatalf("expected 2 variables, got %d", len(result.Table.Variables))
		}
		if result.Table.Variables[0].Name != "Blue" {
			t.Errorf("expected variable name Blue, got %s", result.Table.Variables[0].Name)
		}
		if result.Table.Variables[0].Value != "14" {
			t.Errorf("expected variable value 14, got %s", result.Table.Variables[0].Value)
		}

		if len(result.Table.ChildTables) != 1 {
			t.Fatalf("expected 1 child table, got %d", len(result.Table.ChildTables))
		}
		if result.Table.ChildTables[0].DBID != "bddrydqhg" {
			t.Errorf("expected child dbid bddrydqhg, got %s", result.Table.ChildTables[0].DBID)
		}
	})

	t.Run("table level schema with fields", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
 <action>API_GetSchema</action>
 <errcode>0</errcode>
 <errtext>No error</errtext>
 <time_zone>(UTC-08:00) Pacific Time (US &amp; Canada)</time_zone>
 <date_format>YYYY-MM-DD</date_format>
 <table>
   <name>Contacts</name>
   <desc>Contact management table.</desc>
   <original>
     <table_id>bdb5rjd6g</table_id>
     <app_id>bdb5rjd6h</app_id>
   </original>
   <queries>
    <query id="1">
     <qyname>List All</qyname>
      <qytype>table</qytype>
     </query>
   </queries>
   <fields>
     <field id="6" field_type="text" base_type="text">
       <label>Name</label>
       <fieldhelp>Enter the contact name</fieldhelp>
       <required>1</required>
     </field>
     <field id="7" field_type="email" base_type="text">
       <label>Email</label>
       <unique>1</unique>
     </field>
   </fields>
 </table>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetSchema(context.Background(), "bdb5rjd6g")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Table.Name != "Contacts" {
			t.Errorf("expected table name Contacts, got %s", result.Table.Name)
		}

		// Check queries
		if len(result.Table.Queries) != 1 {
			t.Fatalf("expected 1 query, got %d", len(result.Table.Queries))
		}
		if result.Table.Queries[0].Name != "List All" {
			t.Errorf("expected query name 'List All', got %s", result.Table.Queries[0].Name)
		}

		// Check fields
		if len(result.Table.Fields) != 2 {
			t.Fatalf("expected 2 fields, got %d", len(result.Table.Fields))
		}

		field1 := result.Table.Fields[0]
		if field1.ID != 6 {
			t.Errorf("expected field ID 6, got %d", field1.ID)
		}
		if field1.Label != "Name" {
			t.Errorf("expected field label Name, got %s", field1.Label)
		}
		if field1.FieldType != "text" {
			t.Errorf("expected field type text, got %s", field1.FieldType)
		}
		if field1.Required != 1 {
			t.Errorf("expected required=1, got %d", field1.Required)
		}
		if field1.FieldHelp != "Enter the contact name" {
			t.Errorf("expected field help 'Enter the contact name', got %s", field1.FieldHelp)
		}

		field2 := result.Table.Fields[1]
		if field2.FieldType != "email" {
			t.Errorf("expected field type email, got %s", field2.FieldType)
		}
		if field2.Unique != 1 {
			t.Errorf("expected unique=1, got %d", field2.Unique)
		}
	})

	t.Run("not found error", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetSchema</action>
   <errcode>6</errcode>
   <errtext>No such database</errtext>
</qdbapi>`),
		}

		client := New(mock)
		_, err := client.GetSchema(context.Background(), "nonexistent")

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !IsNotFound(err) {
			t.Errorf("expected IsNotFound to return true, got false for: %v", err)
		}
	})
}

func TestErrorHelpers(t *testing.T) {
	tests := []struct {
		name           string
		code           int
		isUnauthorized bool
		isNotFound     bool
		isInvalidTkt   bool
	}{
		{"unauthorized", ErrCodeUnauthorized, true, false, false},
		{"access denied", ErrCodeAccessDenied, true, false, false},
		{"no such database", ErrCodeNoSuchDatabase, false, true, false},
		{"no such record", ErrCodeNoSuchRecord, false, true, false},
		{"no such field", ErrCodeNoSuchField, false, true, false},
		{"no such user", ErrCodeNoSuchUser, false, true, false},
		{"invalid ticket", ErrCodeInvalidTicket, false, false, true},
		{"success", ErrCodeSuccess, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Code: tt.code, Text: tt.name}

			if IsUnauthorized(err) != tt.isUnauthorized {
				t.Errorf("IsUnauthorized: expected %v, got %v", tt.isUnauthorized, IsUnauthorized(err))
			}
			if IsNotFound(err) != tt.isNotFound {
				t.Errorf("IsNotFound: expected %v, got %v", tt.isNotFound, IsNotFound(err))
			}
			if IsInvalidTicket(err) != tt.isInvalidTkt {
				t.Errorf("IsInvalidTicket: expected %v, got %v", tt.isInvalidTkt, IsInvalidTicket(err))
			}
		})
	}

	// Test with non-Error type
	t.Run("non-Error type", func(t *testing.T) {
		err := errors.New("generic error")
		if IsUnauthorized(err) {
			t.Error("IsUnauthorized should return false for non-Error")
		}
		if IsNotFound(err) {
			t.Error("IsNotFound should return false for non-Error")
		}
		if IsInvalidTicket(err) {
			t.Error("IsInvalidTicket should return false for non-Error")
		}
	})
}

func TestBuildRequest(t *testing.T) {
	tests := []struct {
		name     string
		inner    string
		expected string
	}{
		{"empty", "", "<qdbapi></qdbapi>"},
		{"with content", "<userid>123</userid>", "<qdbapi><userid>123</userid></qdbapi>"},
		{"multiple elements", "<foo>1</foo><bar>2</bar>", "<qdbapi><foo>1</foo><bar>2</bar></qdbapi>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(buildRequest(tt.inner))
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDoQueryCount(t *testing.T) {
	t.Run("success with query", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_DoQueryCount</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <numMatches>42</numMatches>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.DoQueryCount(context.Background(), "bqxyz123", "{'7'.EX.'Active'}")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_DoQueryCount" {
			t.Errorf("expected action API_DoQueryCount, got %s", mock.lastAction)
		}

		if mock.lastDBID != "bqxyz123" {
			t.Errorf("expected dbid bqxyz123, got %s", mock.lastDBID)
		}

		// Verify request body contains query
		body := string(mock.lastBody)
		if body != "<qdbapi><query>{'7'.EX.'Active'}</query></qdbapi>" {
			t.Errorf("unexpected request body: %s", body)
		}

		if result.NumMatches != 42 {
			t.Errorf("expected 42 matches, got %d", result.NumMatches)
		}
	})

	t.Run("success without query (all records)", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_DoQueryCount</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <numMatches>1000</numMatches>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.DoQueryCount(context.Background(), "bqxyz123", "")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify request body has no query element
		body := string(mock.lastBody)
		if body != "<qdbapi></qdbapi>" {
			t.Errorf("unexpected request body: %s", body)
		}

		if result.NumMatches != 1000 {
			t.Errorf("expected 1000 matches, got %d", result.NumMatches)
		}
	})

	t.Run("error response", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_DoQueryCount</action>
   <errcode>6</errcode>
   <errtext>No such database</errtext>
</qdbapi>`),
		}

		client := New(mock)
		_, err := client.DoQueryCount(context.Background(), "nonexistent", "")

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !IsNotFound(err) {
			t.Errorf("expected IsNotFound to return true")
		}
	})
}

func TestGetRecordInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_getrecordinfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <rid>20</rid>
   <num_fields>3</num_fields>
   <update_id>1205780029699</update_id>
   <field>
      <fid>3</fid>
      <name>Record ID#</name>
      <type>Record ID#</type>
      <value>20</value>
   </field>
   <field>
      <fid>6</fid>
      <name>Start Date</name>
      <type>Date</type>
      <value>1437609600000</value>
      <printable>07-23-2015</printable>
   </field>
   <field>
      <fid>7</fid>
      <name>Status</name>
      <type>Text</type>
      <value>Active</value>
   </field>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetRecordInfo(context.Background(), "bqxyz123", 20)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetRecordInfo" {
			t.Errorf("expected action API_GetRecordInfo, got %s", mock.lastAction)
		}

		// Verify request body contains rid
		body := string(mock.lastBody)
		if body != "<qdbapi><rid>20</rid></qdbapi>" {
			t.Errorf("unexpected request body: %s", body)
		}

		if result.RecordID != 20 {
			t.Errorf("expected record ID 20, got %d", result.RecordID)
		}
		if result.NumFields != 3 {
			t.Errorf("expected 3 fields, got %d", result.NumFields)
		}
		if result.UpdateID != "1205780029699" {
			t.Errorf("expected update ID 1205780029699, got %s", result.UpdateID)
		}

		if len(result.Fields) != 3 {
			t.Fatalf("expected 3 fields, got %d", len(result.Fields))
		}

		// Check Record ID field
		field0 := result.Fields[0]
		if field0.ID != 3 {
			t.Errorf("expected field ID 3, got %d", field0.ID)
		}
		if field0.Name != "Record ID#" {
			t.Errorf("expected field name 'Record ID#', got %s", field0.Name)
		}
		if field0.Type != "Record ID#" {
			t.Errorf("expected field type 'Record ID#', got %s", field0.Type)
		}
		if field0.Value != "20" {
			t.Errorf("expected value '20', got %s", field0.Value)
		}

		// Check Date field with printable
		field1 := result.Fields[1]
		if field1.ID != 6 {
			t.Errorf("expected field ID 6, got %d", field1.ID)
		}
		if field1.Name != "Start Date" {
			t.Errorf("expected field name 'Start Date', got %s", field1.Name)
		}
		if field1.Type != "Date" {
			t.Errorf("expected field type 'Date', got %s", field1.Type)
		}
		if field1.Value != "1437609600000" {
			t.Errorf("expected value '1437609600000', got %s", field1.Value)
		}
		if field1.Printable != "07-23-2015" {
			t.Errorf("expected printable '07-23-2015', got %s", field1.Printable)
		}

		// Check Text field
		field2 := result.Fields[2]
		if field2.Value != "Active" {
			t.Errorf("expected value 'Active', got %s", field2.Value)
		}
		if field2.Printable != "" {
			t.Errorf("expected empty printable, got %s", field2.Printable)
		}
	})

	t.Run("record not found", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_getrecordinfo</action>
   <errcode>30</errcode>
   <errtext>No such record</errtext>
</qdbapi>`),
		}

		client := New(mock)
		_, err := client.GetRecordInfo(context.Background(), "bqxyz123", 99999)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !IsNotFound(err) {
			t.Errorf("expected IsNotFound to return true")
		}
	})
}

func TestGetRecordInfoByKey(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_getrecordinfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <rid>42</rid>
   <num_fields>2</num_fields>
   <update_id>1234567890</update_id>
   <field>
      <fid>6</fid>
      <name>Order Number</name>
      <type>Text</type>
      <value>ORD-12345</value>
   </field>
   <field>
      <fid>7</fid>
      <name>Status</name>
      <type>Text</type>
      <value>Shipped</value>
   </field>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetRecordInfoByKey(context.Background(), "bqxyz123", "ORD-12345")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify request body contains key
		body := string(mock.lastBody)
		if body != "<qdbapi><key>ORD-12345</key></qdbapi>" {
			t.Errorf("unexpected request body: %s", body)
		}

		if result.RecordID != 42 {
			t.Errorf("expected record ID 42, got %d", result.RecordID)
		}

		if len(result.Fields) != 2 {
			t.Fatalf("expected 2 fields, got %d", len(result.Fields))
		}

		if result.Fields[0].Value != "ORD-12345" {
			t.Errorf("expected value 'ORD-12345', got %s", result.Fields[0].Value)
		}
	})
}

func TestGrantedDBs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GrantedDBs</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <databases>
      <dbinfo>
         <dbname>MyApp</dbname>
         <dbid>bdzk2ecg5</dbid>
         <ancestorappid>beaa6db7t</ancestorappid>
         <oldestancestorappid>bd9jbshim</oldestancestorappid>
      </dbinfo>
      <dbinfo>
         <dbname>MyApp:Table1</dbname>
         <dbid>bdzuh4nj5</dbid>
      </dbinfo>
   </databases>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GrantedDBs(context.Background(), GrantedDBsOptions{
			RealmAppsOnly:    true,
			IncludeAncestors: true,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GrantedDBs" {
			t.Errorf("expected action API_GrantedDBs, got %s", mock.lastAction)
		}

		if mock.lastDBID != "main" {
			t.Errorf("expected dbid main, got %s", mock.lastDBID)
		}

		if len(result.Databases) != 2 {
			t.Fatalf("expected 2 databases, got %d", len(result.Databases))
		}

		db1 := result.Databases[0]
		if db1.DBID != "bdzk2ecg5" {
			t.Errorf("expected dbid bdzk2ecg5, got %s", db1.DBID)
		}
		if db1.Name != "MyApp" {
			t.Errorf("expected name MyApp, got %s", db1.Name)
		}
		if db1.AncestorAppID != "beaa6db7t" {
			t.Errorf("expected ancestorappid beaa6db7t, got %s", db1.AncestorAppID)
		}

		db2 := result.Databases[1]
		if db2.Name != "MyApp:Table1" {
			t.Errorf("expected name MyApp:Table1, got %s", db2.Name)
		}
	})
}

func TestFindDBByName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_FindDBByName</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <dbid>bdcagynhs</dbid>
   <dbname>TestApp</dbname>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.FindDBByName(context.Background(), "TestApp", true)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_FindDBByName" {
			t.Errorf("expected action API_FindDBByName, got %s", mock.lastAction)
		}

		if result.DBID != "bdcagynhs" {
			t.Errorf("expected dbid bdcagynhs, got %s", result.DBID)
		}
		if result.Name != "TestApp" {
			t.Errorf("expected name TestApp, got %s", result.Name)
		}
	})
}

func TestGetDBInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetDBInfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <dbname>test</dbname>
   <lastRecModTime>1205806751959</lastRecModTime>
   <lastModifiedTime>1205877093679</lastModifiedTime>
   <createdTime>1204745351407</createdTime>
   <numRecords>42</numRecords>
   <mgrID>112149.bhsv</mgrID>
   <mgrName>AppBoss</mgrName>
   <time_zone>(UTC-08:00) Pacific Time (US &amp; Canada)</time_zone>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetDBInfo(context.Background(), "bqxyz123")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetDBInfo" {
			t.Errorf("expected action API_GetDBInfo, got %s", mock.lastAction)
		}

		if result.Name != "test" {
			t.Errorf("expected name test, got %s", result.Name)
		}
		if result.NumRecords != 42 {
			t.Errorf("expected 42 records, got %d", result.NumRecords)
		}
		if result.ManagerID != "112149.bhsv" {
			t.Errorf("expected manager ID 112149.bhsv, got %s", result.ManagerID)
		}
		if result.ManagerName != "AppBoss" {
			t.Errorf("expected manager name AppBoss, got %s", result.ManagerName)
		}
		if result.LastRecModTime != 1205806751959 {
			t.Errorf("expected lastRecModTime 1205806751959, got %d", result.LastRecModTime)
		}
	})
}

func TestGetNumRecords(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetNumRecords</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <num_records>17</num_records>
</qdbapi>`),
		}

		client := New(mock)
		count, err := client.GetNumRecords(context.Background(), "bqxyz123")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetNumRecords" {
			t.Errorf("expected action API_GetNumRecords, got %s", mock.lastAction)
		}

		if count != 17 {
			t.Errorf("expected 17 records, got %d", count)
		}
	})
}

func TestGetUserInfo(t *testing.T) {
	t.Run("success with email", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_getuserinfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <user id="112149.bhsv">
      <firstName>Ragnar</firstName>
      <lastName>Lodbrok</lastName>
      <login>Ragnar</login>
      <email>Ragnar-Lodbrok@paris.net</email>
      <screenName>Ragnar</screenName>
      <isVerified>1</isVerified>
      <externalAuth>0</externalAuth>
   </user>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetUserInfo(context.Background(), "Ragnar-Lodbrok@paris.net")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetUserInfo" {
			t.Errorf("expected action API_GetUserInfo, got %s", mock.lastAction)
		}

		if result.ID != "112149.bhsv" {
			t.Errorf("expected ID 112149.bhsv, got %s", result.ID)
		}
		if result.FirstName != "Ragnar" {
			t.Errorf("expected firstName Ragnar, got %s", result.FirstName)
		}
		if result.LastName != "Lodbrok" {
			t.Errorf("expected lastName Lodbrok, got %s", result.LastName)
		}
		if result.Email != "Ragnar-Lodbrok@paris.net" {
			t.Errorf("expected email Ragnar-Lodbrok@paris.net, got %s", result.Email)
		}
		if !result.IsVerified {
			t.Error("expected IsVerified to be true")
		}
		if result.ExternalAuth {
			t.Error("expected ExternalAuth to be false")
		}
	})
}

func TestGetDBVar(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_getDBvar</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <value>42</value>
</qdbapi>`),
		}

		client := New(mock)
		value, err := client.GetDBVar(context.Background(), "bqxyz123", "myVar")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetDBVar" {
			t.Errorf("expected action API_GetDBVar, got %s", mock.lastAction)
		}

		if value != "42" {
			t.Errorf("expected value 42, got %s", value)
		}
	})
}

func TestSetDBVar(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_SetDBVar</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.SetDBVar(context.Background(), "bqxyz123", "myVar", "newValue")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_SetDBVar" {
			t.Errorf("expected action API_SetDBVar, got %s", mock.lastAction)
		}

		// Verify request body
		body := string(mock.lastBody)
		expected := "<qdbapi><varname>myVar</varname><value>newValue</value></qdbapi>"
		if body != expected {
			t.Errorf("expected body %s, got %s", expected, body)
		}
	})
}

func TestAddUserToRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_AddUserToRole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.AddUserToRole(context.Background(), "bqxyz123", "112149.bhsv", 10)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_AddUserToRole" {
			t.Errorf("expected action API_AddUserToRole, got %s", mock.lastAction)
		}

		// Verify request body
		body := string(mock.lastBody)
		expected := "<qdbapi><userid>112149.bhsv</userid><roleid>10</roleid></qdbapi>"
		if body != expected {
			t.Errorf("expected body %s, got %s", expected, body)
		}
	})
}

func TestRemoveUserFromRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_RemoveUserFromRole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.RemoveUserFromRole(context.Background(), "bqxyz123", "112149.bhsv", 10)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_RemoveUserFromRole" {
			t.Errorf("expected action API_RemoveUserFromRole, got %s", mock.lastAction)
		}
	})
}

func TestChangeUserRole(t *testing.T) {
	t.Run("change to new role", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_changeuserrole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.ChangeUserRole(context.Background(), "bqxyz123", "112149.bhsv", 10, 11)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_ChangeUserRole" {
			t.Errorf("expected action API_ChangeUserRole, got %s", mock.lastAction)
		}

		// Verify request body
		body := string(mock.lastBody)
		expected := "<qdbapi><userid>112149.bhsv</userid><roleid>10</roleid><newRoleid>11</newRoleid></qdbapi>"
		if body != expected {
			t.Errorf("expected body %s, got %s", expected, body)
		}
	})

	t.Run("disable access (role=None)", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_changeuserrole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.ChangeUserRole(context.Background(), "bqxyz123", "112149.bhsv", 10, 0)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify request body does NOT contain newRoleid
		body := string(mock.lastBody)
		expected := "<qdbapi><userid>112149.bhsv</userid><roleid>10</roleid></qdbapi>"
		if body != expected {
			t.Errorf("expected body %s, got %s", expected, body)
		}
	})
}

// Group Management Tests

func TestCreateGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_CreateGroup</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <group id="1217.dgpt">
      <name>MarketingSupport</name>
      <description>Support staff for sr marketing group</description>
      <managedByUser>true</managedByUser>
   </group>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.CreateGroup(context.Background(), "MarketingSupport", "Support staff for sr marketing group", "")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_CreateGroup" {
			t.Errorf("expected action API_CreateGroup, got %s", mock.lastAction)
		}

		if result.Group.ID != "1217.dgpt" {
			t.Errorf("expected group ID 1217.dgpt, got %s", result.Group.ID)
		}

		if result.Group.Name != "MarketingSupport" {
			t.Errorf("expected group name MarketingSupport, got %s", result.Group.Name)
		}
	})
}

func TestDeleteGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_DeleteGroup</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.DeleteGroup(context.Background(), "1217.dgpt")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_DeleteGroup" {
			t.Errorf("expected action API_DeleteGroup, got %s", mock.lastAction)
		}
	})
}

func TestGetUsersInGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetUsersInGroup</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <group id="2345.sdfk">
      <name>GroupInfoTestGroup</name>
      <description>My Group description</description>
      <users>
         <user id="112149.bhsv">
            <firstName>John</firstName>
            <lastName>Doe</lastName>
            <email>jdoe@example.com</email>
            <screenName></screenName>
            <isAdmin>false</isAdmin>
         </user>
      </users>
      <managers>
         <manager id="52731770.b82h">
            <firstName>Angela</firstName>
            <lastName>Leon</lastName>
            <email>angela@example.com</email>
            <screenName>aqleon</screenName>
            <isMember>true</isMember>
         </manager>
      </managers>
      <subgroups>
         <subgroup id="3450.aefs"/>
      </subgroups>
   </group>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetUsersInGroup(context.Background(), "2345.sdfk", true)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Name != "GroupInfoTestGroup" {
			t.Errorf("expected group name GroupInfoTestGroup, got %s", result.Name)
		}

		if len(result.Users) != 1 {
			t.Fatalf("expected 1 user, got %d", len(result.Users))
		}

		if result.Users[0].FirstName != "John" {
			t.Errorf("expected first name John, got %s", result.Users[0].FirstName)
		}

		if len(result.Managers) != 1 {
			t.Fatalf("expected 1 manager, got %d", len(result.Managers))
		}

		if result.Managers[0].FirstName != "Angela" {
			t.Errorf("expected manager first name Angela, got %s", result.Managers[0].FirstName)
		}

		if len(result.Subgroups) != 1 {
			t.Fatalf("expected 1 subgroup, got %d", len(result.Subgroups))
		}
	})
}

func TestAddUserToGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_AddUserToGroup</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.AddUserToGroup(context.Background(), "1217.dgpt", "112149.bhsv", true)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_AddUserToGroup" {
			t.Errorf("expected action API_AddUserToGroup, got %s", mock.lastAction)
		}
	})
}

func TestRemoveUserFromGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_RemoveUserFromGroup</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.RemoveUserFromGroup(context.Background(), "1217.dgpt", "112149.bhsv")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_RemoveUserFromGroup" {
			t.Errorf("expected action API_RemoveUserFromGroup, got %s", mock.lastAction)
		}
	})
}

func TestGetGroupRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetGroupRole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <roles>
      <role id="11">
         <name>Participant</name>
      </role>
   </roles>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.GetGroupRole(context.Background(), "bqxyz123", "1217.dgpt")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Roles) != 1 {
			t.Fatalf("expected 1 role, got %d", len(result.Roles))
		}

		if result.Roles[0].ID != 11 {
			t.Errorf("expected role ID 11, got %d", result.Roles[0].ID)
		}
	})
}

func TestAddGroupToRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_AddGroupToRole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.AddGroupToRole(context.Background(), "bqxyz123", "1217.dgpt", 12)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_AddGroupToRole" {
			t.Errorf("expected action API_AddGroupToRole, got %s", mock.lastAction)
		}
	})
}

func TestRemoveGroupFromRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_RemoveGroupFromRole</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.RemoveGroupFromRole(context.Background(), "bqxyz123", "1217.dgpt", 12, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_RemoveGroupFromRole" {
			t.Errorf("expected action API_RemoveGroupFromRole, got %s", mock.lastAction)
		}
	})
}

// Code Pages Tests

func TestGetDBPage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm:    "testrealm",
			response: []byte(`<html><body>Hello World</body></html>`),
		}

		client := New(mock)
		content, err := client.GetDBPage(context.Background(), "bqxyz123", "3")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_GetDBPage" {
			t.Errorf("expected action API_GetDBPage, got %s", mock.lastAction)
		}

		if content != "<html><body>Hello World</body></html>" {
			t.Errorf("unexpected content: %s", content)
		}
	})

	t.Run("error response", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetDBPage</action>
   <errcode>24</errcode>
   <errtext>No such page</errtext>
</qdbapi>`),
		}

		client := New(mock)
		_, err := client.GetDBPage(context.Background(), "bqxyz123", "999")

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestAddReplaceDBPage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_AddReplaceDBPage</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <pageID>6</pageID>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.AddReplaceDBPage(context.Background(), "bqxyz123", "newpage.html", 0, PageTypeXSLOrHTML, "<html><body>Hello</body></html>")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_AddReplaceDBPage" {
			t.Errorf("expected action API_AddReplaceDBPage, got %s", mock.lastAction)
		}

		if result.PageID != 6 {
			t.Errorf("expected page ID 6, got %d", result.PageID)
		}
	})

	t.Run("error no name or id", func(t *testing.T) {
		mock := &mockCaller{realm: "testrealm"}
		client := New(mock)

		_, err := client.AddReplaceDBPage(context.Background(), "bqxyz123", "", 0, PageTypeXSLOrHTML, "content")

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// User Provisioning Tests

func TestProvisionUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_provisionuser</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <userid>112248.5nzg</userid>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.ProvisionUser(context.Background(), "bqxyz123", "new@example.com", "John", "Doe", 11)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_ProvisionUser" {
			t.Errorf("expected action API_ProvisionUser, got %s", mock.lastAction)
		}

		if result.UserID != "112248.5nzg" {
			t.Errorf("expected user ID 112248.5nzg, got %s", result.UserID)
		}
	})
}

func TestSendInvitation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_SendInvitation</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.SendInvitation(context.Background(), "bqxyz123", "112149.bhsv", "Welcome!")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_SendInvitation" {
			t.Errorf("expected action API_SendInvitation, got %s", mock.lastAction)
		}
	})
}

func TestChangeManager(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_ChangeManager</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.ChangeManager(context.Background(), "bqxyz123", "newmanager@example.com")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_ChangeManager" {
			t.Errorf("expected action API_ChangeManager, got %s", mock.lastAction)
		}
	})
}

func TestChangeRecordOwner(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_changerecordowner</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.ChangeRecordOwner(context.Background(), "bqxyz123", 123, "newowner@example.com")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_ChangeRecordOwner" {
			t.Errorf("expected action API_ChangeRecordOwner, got %s", mock.lastAction)
		}
	})
}

// Field Management Tests

func TestFieldAddChoices(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_FieldAddChoices</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <fid>11</fid>
   <fname>Color</fname>
   <numadded>3</numadded>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.FieldAddChoices(context.Background(), "bqxyz123", 11, []string{"Red", "Green", "Blue"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_FieldAddChoices" {
			t.Errorf("expected action API_FieldAddChoices, got %s", mock.lastAction)
		}

		if result.FieldID != 11 {
			t.Errorf("expected field ID 11, got %d", result.FieldID)
		}

		if result.NumAdded != 3 {
			t.Errorf("expected 3 added, got %d", result.NumAdded)
		}
	})
}

func TestFieldRemoveChoices(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_FieldRemoveChoices</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <fid>11</fid>
   <fname>Color</fname>
   <numremoved>2</numremoved>
</qdbapi>`),
		}

		client := New(mock)
		result, err := client.FieldRemoveChoices(context.Background(), "bqxyz123", 11, []string{"Red", "Blue"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_FieldRemoveChoices" {
			t.Errorf("expected action API_FieldRemoveChoices, got %s", mock.lastAction)
		}

		if result.NumRemoved != 2 {
			t.Errorf("expected 2 removed, got %d", result.NumRemoved)
		}
	})
}

func TestSetKeyField(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_SetKeyField</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
</qdbapi>`),
		}

		client := New(mock)
		err := client.SetKeyField(context.Background(), "bqxyz123", 6)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastAction != "API_SetKeyField" {
			t.Errorf("expected action API_SetKeyField, got %s", mock.lastAction)
		}
	})
}

// =============================================================================
// Schema Helper Tests
// =============================================================================

func TestWithSchema(t *testing.T) {
	t.Run("resolves table alias in API call", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetDBInfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <dbname>Projects</dbname>
   <numRecords>10</numRecords>
   <mgrID>1234</mgrID>
   <mgrName>Manager</mgrName>
</qdbapi>`),
		}

		schema := core.ResolveSchema(&core.Schema{
			Tables: map[string]core.TableSchema{
				"projects": {ID: "bqxyz123", Fields: map[string]int{"name": 6, "status": 7}},
			},
		})

		client := New(mock, WithSchema(schema))
		_, err := client.GetDBInfo(context.Background(), "projects")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify the resolved ID was used in the API call
		if mock.lastDBID != "bqxyz123" {
			t.Errorf("expected resolved dbid bqxyz123, got %s", mock.lastDBID)
		}
	})

	t.Run("passes through unknown alias as literal ID", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetDBInfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <dbname>Test</dbname>
   <numRecords>0</numRecords>
</qdbapi>`),
		}

		schema := core.ResolveSchema(&core.Schema{
			Tables: map[string]core.TableSchema{},
		})

		client := New(mock, WithSchema(schema))
		_, err := client.GetDBInfo(context.Background(), "literal_dbid")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.lastDBID != "literal_dbid" {
			t.Errorf("expected literal dbid, got %s", mock.lastDBID)
		}
	})
}

func TestGetRecordInfoResultField(t *testing.T) {
	t.Run("access field by alias", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_getrecordinfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <rid>20</rid>
   <num_fields>2</num_fields>
   <update_id>1234567890</update_id>
   <field>
      <fid>6</fid>
      <name>Name</name>
      <type>Text</type>
      <value>Test Project</value>
   </field>
   <field>
      <fid>7</fid>
      <name>Status</name>
      <type>Text</type>
      <value>Active</value>
   </field>
</qdbapi>`),
		}

		schema := core.ResolveSchema(&core.Schema{
			Tables: map[string]core.TableSchema{
				"projects": {ID: "bqxyz123", Fields: map[string]int{"name": 6, "status": 7}},
			},
		})

		client := New(mock, WithSchema(schema))
		result, err := client.GetRecordInfo(context.Background(), "projects", 20)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Test Field helper with alias
		nameField := result.Field("name")
		if nameField == nil {
			t.Fatal("expected to find 'name' field, got nil")
		}
		if nameField.Value != "Test Project" {
			t.Errorf("expected value 'Test Project', got %s", nameField.Value)
		}

		statusField := result.Field("status")
		if statusField == nil {
			t.Fatal("expected to find 'status' field, got nil")
		}
		if statusField.Value != "Active" {
			t.Errorf("expected value 'Active', got %s", statusField.Value)
		}

		// Test Field helper with ID as string
		field6 := result.Field("6")
		if field6 == nil {
			t.Fatal("expected to find field '6', got nil")
		}
		if field6.Value != "Test Project" {
			t.Errorf("expected value 'Test Project', got %s", field6.Value)
		}

		// Test FieldByID helper
		field7 := result.FieldByID(7)
		if field7 == nil {
			t.Fatal("expected to find field ID 7, got nil")
		}
		if field7.Value != "Active" {
			t.Errorf("expected value 'Active', got %s", field7.Value)
		}

		// Test unknown field returns nil
		unknown := result.Field("unknown")
		if unknown != nil {
			t.Errorf("expected nil for unknown field, got %v", unknown)
		}
	})

	t.Run("access field by ID without schema", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>api_getrecordinfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <rid>20</rid>
   <num_fields>1</num_fields>
   <update_id>1234567890</update_id>
   <field>
      <fid>6</fid>
      <name>Name</name>
      <type>Text</type>
      <value>Test</value>
   </field>
</qdbapi>`),
		}

		client := New(mock) // No schema
		result, err := client.GetRecordInfo(context.Background(), "bqxyz123", 20)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Access by ID string works without schema
		field := result.Field("6")
		if field == nil {
			t.Fatal("expected to find field '6', got nil")
		}
		if field.Value != "Test" {
			t.Errorf("expected value 'Test', got %s", field.Value)
		}

		// FieldByID works without schema
		fieldByID := result.FieldByID(6)
		if fieldByID == nil {
			t.Fatal("expected to find field ID 6, got nil")
		}
	})
}

func TestGrantedDBsResultDatabase(t *testing.T) {
	t.Run("access database by alias", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GrantedDBs</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <databases>
      <dbinfo>
         <dbname>Projects</dbname>
         <dbid>bqxyz123</dbid>
      </dbinfo>
      <dbinfo>
         <dbname>Tasks</dbname>
         <dbid>bqabc456</dbid>
      </dbinfo>
   </databases>
</qdbapi>`),
		}

		schema := core.ResolveSchema(&core.Schema{
			Tables: map[string]core.TableSchema{
				"projects": {ID: "bqxyz123", Fields: map[string]int{}},
				"tasks":    {ID: "bqabc456", Fields: map[string]int{}},
			},
		})

		client := New(mock, WithSchema(schema))
		result, err := client.GrantedDBs(context.Background(), GrantedDBsOptions{})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Access by alias
		projects := result.Database("projects")
		if projects == nil {
			t.Fatal("expected to find 'projects' database, got nil")
		}
		if projects.Name != "Projects" {
			t.Errorf("expected name 'Projects', got %s", projects.Name)
		}

		// Access by DBID
		tasks := result.Database("bqabc456")
		if tasks == nil {
			t.Fatal("expected to find tasks by DBID, got nil")
		}
		if tasks.Name != "Tasks" {
			t.Errorf("expected name 'Tasks', got %s", tasks.Name)
		}

		// Unknown returns nil
		unknown := result.Database("unknown")
		if unknown != nil {
			t.Errorf("expected nil for unknown database, got %v", unknown)
		}
	})
}

func TestGetAppDTMInfoResultTable(t *testing.T) {
	t.Run("access table by alias", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetAppDTMInfo</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <RequestTime>1234567890</RequestTime>
   <RequestNextAllowedTime>1234567895</RequestNextAllowedTime>
   <app id="bqapp123">
      <lastModifiedTime>1234567800</lastModifiedTime>
      <lastRecModTime>1234567700</lastRecModTime>
   </app>
   <tables>
      <table id="bqtable1">
         <lastModifiedTime>1234567600</lastModifiedTime>
         <lastRecModTime>1234567500</lastRecModTime>
      </table>
      <table id="bqtable2">
         <lastModifiedTime>1234567400</lastModifiedTime>
         <lastRecModTime>1234567300</lastRecModTime>
      </table>
   </tables>
</qdbapi>`),
		}

		schema := core.ResolveSchema(&core.Schema{
			Tables: map[string]core.TableSchema{
				"projects": {ID: "bqtable1", Fields: map[string]int{}},
				"tasks":    {ID: "bqtable2", Fields: map[string]int{}},
			},
		})

		client := New(mock, WithSchema(schema))
		result, err := client.GetAppDTMInfo(context.Background(), "bqapp123")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Access by alias
		projects := result.Table("projects")
		if projects == nil {
			t.Fatal("expected to find 'projects' table, got nil")
		}
		if projects.ID != "bqtable1" {
			t.Errorf("expected ID 'bqtable1', got %s", projects.ID)
		}

		// Access by ID
		table2 := result.Table("bqtable2")
		if table2 == nil {
			t.Fatal("expected to find table by ID, got nil")
		}
		if table2.ID != "bqtable2" {
			t.Errorf("expected ID 'bqtable2', got %s", table2.ID)
		}

		// Unknown returns nil
		unknown := result.Table("unknown")
		if unknown != nil {
			t.Errorf("expected nil for unknown table, got %v", unknown)
		}
	})
}

func TestSchemaResultField(t *testing.T) {
	t.Run("access field by alias", func(t *testing.T) {
		mock := &mockCaller{
			realm: "testrealm",
			response: []byte(`<?xml version="1.0" ?>
<qdbapi>
   <action>API_GetSchema</action>
   <errcode>0</errcode>
   <errtext>No error</errtext>
   <time_zone>(UTC-08:00) Pacific</time_zone>
   <date_format>MM-DD-YYYY</date_format>
   <table>
      <name>Projects</name>
      <fields>
         <field id="6" field_type="text" base_type="text">
            <label>Name</label>
         </field>
         <field id="7" field_type="text" base_type="text">
            <label>Status</label>
         </field>
      </fields>
      <chdbids>
         <chdbid name="_dbid_tasks">bqtasks123</chdbid>
      </chdbids>
   </table>
</qdbapi>`),
		}

		schema := core.ResolveSchema(&core.Schema{
			Tables: map[string]core.TableSchema{
				"projects": {ID: "bqxyz123", Fields: map[string]int{"name": 6, "status": 7}},
				"tasks":    {ID: "bqtasks123", Fields: map[string]int{}},
			},
		})

		client := New(mock, WithSchema(schema))
		result, err := client.GetSchema(context.Background(), "projects")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify table alias was resolved
		if mock.lastDBID != "bqxyz123" {
			t.Errorf("expected resolved dbid bqxyz123, got %s", mock.lastDBID)
		}

		// Access field by alias
		nameField := result.Field("name")
		if nameField == nil {
			t.Fatal("expected to find 'name' field, got nil")
		}
		if nameField.Label != "Name" {
			t.Errorf("expected label 'Name', got %s", nameField.Label)
		}

		// Access field by ID string
		field7 := result.Field("7")
		if field7 == nil {
			t.Fatal("expected to find field '7', got nil")
		}
		if field7.Label != "Status" {
			t.Errorf("expected label 'Status', got %s", field7.Label)
		}

		// Access field by ID
		fieldByID := result.FieldByID(6)
		if fieldByID == nil {
			t.Fatal("expected to find field ID 6, got nil")
		}

		// Access child table by alias
		tasksTable := result.ChildTable("tasks")
		if tasksTable == nil {
			t.Fatal("expected to find 'tasks' child table, got nil")
		}
		if tasksTable.DBID != "bqtasks123" {
			t.Errorf("expected DBID 'bqtasks123', got %s", tasksTable.DBID)
		}

		// Access child table by DBID
		childByID := result.ChildTable("bqtasks123")
		if childByID == nil {
			t.Fatal("expected to find child table by DBID, got nil")
		}

		// Unknown returns nil
		unknownField := result.Field("unknown")
		if unknownField != nil {
			t.Errorf("expected nil for unknown field, got %v", unknownField)
		}

		unknownChild := result.ChildTable("unknown")
		if unknownChild != nil {
			t.Errorf("expected nil for unknown child table, got %v", unknownChild)
		}
	})
}
