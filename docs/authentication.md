# Authentication

| Method | Option | Use it for |
|---|---|---|
| [User token](#user-token) | `WithUserToken(token)` | Scripts and services |
| [Ticket](#ticket) | `WithTicketAuth(email, password)` | Acting as a user who signs in with their QuickBase password |
| [Temporary tokens](#temporary-tokens) | `WithTempTokens(map[table]token)` | A server called from a code page |
| [SSO](#sso) | `WithSSOTokenAuth(samlAssertion)` | Acting as a user signed in through your identity provider |
| [App token](#app-tokens) | `WithAppToken(token)` | XML API calls with ticket or temporary-token auth |

## User token

```go
client, _ := quickbase.New("myrealm", quickbase.WithUserToken("b9f3pk_xxx"))
```

Create one at `https://YOUR-REALM.quickbase.com/db/main?a=UserTokens`. Changes made with it are attributed to the token's owner.

## Ticket

A user's email and password, exchanged for a ticket. Records they change show them as the creator or modifier.

```go
client, _ := quickbase.New("myrealm",
	quickbase.WithTicketAuth("user@example.com", "password", auth.WithTicketHours(24*7)),
)
```

- The sign-in happens on the first API call, and the password is cleared from memory right after, whether it worked or not.
- A ticket lasts 12 hours unless `auth.WithTicketHours` says otherwise (1 to 4,380 hours, about six months). When it expires, calls fail; create a new client with fresh credentials.
- `client.SignOut()` clears the ticket from memory. QuickBase has no API to revoke a ticket, so it stays valid on QuickBase's side until it expires.

## Temporary tokens

A temporary token proves a user is signed in to QuickBase. It lasts about five minutes and works for one table. A browser can get one with the user's QuickBase session (a code page, say) and send it to your server, which then calls QuickBase as that user:

```go
handler := func(w http.ResponseWriter, r *http.Request) {
	client, err := quickbase.New("myrealm",
		quickbase.WithTempTokens(map[string]string{
			"bqtable1": r.Header.Get("X-QB-Token-bqtable1"),
		}),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	records, err := client.RunQueryAll(r.Context(), quickbase.RunQueryBody{From: "bqtable1", Select: quickbase.Ints(3, 6)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintf(w, "%d records", len(records))
}
```

Your server stores no credentials, and each call carries the user's own permissions. The header name is yours to choose; the browser side ([quickbase-js](https://github.com/DrewBradfordXYZ/quickbase-js)) can fetch the tokens. `WithTempTokenAuth(opts...)` takes the `auth` package's temporary-token options for more control.

## SSO

With SAML SSO set up for your realm, your server can exchange a SAML assertion from your identity provider (Okta, Entra ID, ...) for a token that acts as that user, using QuickBase's [token exchange](https://developer.quickbase.com/operation/exchangeSsoToken):

```go
client, _ := quickbase.New("myrealm", quickbase.WithSSOTokenAuth("base64url-saml-assertion"))
```

Calls then carry that user's permissions, "Created By" shows them rather than a service account, and there is no long-lived token to leak.

## App tokens

An app with **Require Application Tokens** checks them on XML API calls made with a ticket or temporary token. User tokens skip the check, and the JSON API never uses app tokens.

```go
client, _ := quickbase.New("myrealm",
	quickbase.WithTicketAuth("user@example.com", "password"),
	quickbase.WithAppToken("app-token"),
)
```
