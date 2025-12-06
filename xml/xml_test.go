package xml

import (
	"context"
	"errors"
	"testing"
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
