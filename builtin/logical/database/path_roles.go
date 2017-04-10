package database

import (
	"fmt"
	"time"

	"github.com/hashicorp/vault/builtin/logical/database/dbplugin"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
)

func pathListRoles(b *databaseBackend) *framework.Path {
	return &framework.Path{
		Pattern: "roles/?$",

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ListOperation: b.pathRoleList,
		},

		HelpSynopsis:    pathRoleHelpSyn,
		HelpDescription: pathRoleHelpDesc,
	}
}

func pathRoles(b *databaseBackend) *framework.Path {
	return &framework.Path{
		Pattern: "roles/" + framework.GenericNameRegex("name"),
		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role.",
			},

			"db_name": {
				Type:        framework.TypeString,
				Description: "Name of the database this role acts on.",
			},

			"creation_statements": {
				Type:        framework.TypeString,
				Description: "SQL string to create a user. See help for more info.",
			},

			"revocation_statements": {
				Type: framework.TypeString,
				Description: `Statements to be executed to revoke a user. Must be a semicolon-separated
							string, a base64-encoded semicolon-separated string, a serialized JSON string
							array, or a base64-encoded serialized JSON string array. The '{{name}}' value
							will be substituted.`,
			},
			"renew_statements": {
				Type: framework.TypeString,
				Description: `Statements to be executed to renew a user. Must be a semicolon-separated
							string, a base64-encoded semicolon-separated string, a serialized JSON string
							array, or a base64-encoded serialized JSON string array. The '{{name}}' value
							will be substituted.`,
			},
			"rollback_statements": {
				Type: framework.TypeString,
				Description: `SQL statements to be executed to revoke a user. Must be a semicolon-separated
							string, a base64-encoded semicolon-separated string, a serialized JSON string
							array, or a base64-encoded serialized JSON string array. The '{{name}}' value
							will be substituted.`,
			},

			"default_ttl": {
				Type:        framework.TypeString,
				Description: "Default ttl for role.",
			},

			"max_ttl": {
				Type:        framework.TypeString,
				Description: "Maximum time a credential is valid for",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   b.pathRoleRead,
			logical.UpdateOperation: b.pathRoleCreate,
			logical.DeleteOperation: b.pathRoleDelete,
		},

		HelpSynopsis:    pathRoleHelpSyn,
		HelpDescription: pathRoleHelpDesc,
	}
}

func (b *databaseBackend) pathRoleDelete(req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete("role/" + data.Get("name").(string))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *databaseBackend) pathRoleRead(req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	role, err := b.Role(req.Storage, data.Get("name").(string))
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"creation_statements":   role.Statements.CreationStatements,
			"revocation_statements": role.Statements.RevocationStatements,
			"rollback_statements":   role.Statements.RollbackStatements,
			"renew_statements":      role.Statements.RenewStatements,
			"default_ttl":           role.DefaultTTL.String(),
			"max_ttl":               role.MaxTTL.String(),
		},
	}, nil
}

func (b *databaseBackend) pathRoleList(req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List("role/")
	if err != nil {
		return nil, err
	}

	return logical.ListResponse(entries), nil
}

func (b *databaseBackend) pathRoleCreate(req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	if name == "" {
		return logical.ErrorResponse("Empty role name attribute given"), nil
	}

	dbName := data.Get("db_name").(string)
	if dbName == "" {
		return logical.ErrorResponse("Empty database name attribute given"), nil
	}

	// Get statements
	creationStmts := data.Get("creation_statements").(string)
	revocationStmts := data.Get("revocation_statements").(string)
	rollbackStmts := data.Get("rollback_statements").(string)
	renewStmts := data.Get("renew_statements").(string)

	// Get TTLs
	defaultTTLRaw := data.Get("default_ttl").(string)
	maxTTLRaw := data.Get("max_ttl").(string)

	defaultTTL, err := time.ParseDuration(defaultTTLRaw)
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf(
			"Invalid default_ttl: %s", err)), nil
	}
	maxTTL, err := time.ParseDuration(maxTTLRaw)
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf(
			"Invalid max_ttl: %s", err)), nil
	}

	statements := dbplugin.Statements{
		CreationStatements:   creationStmts,
		RevocationStatements: revocationStmts,
		RollbackStatements:   rollbackStmts,
		RenewStatements:      renewStmts,
	}

	// TODO: Think about preparing the statments to test.

	// Store it
	entry, err := logical.StorageEntryJSON("role/"+name, &roleEntry{
		DBName:     dbName,
		Statements: statements,
		DefaultTTL: defaultTTL,
		MaxTTL:     maxTTL,
	})
	if err != nil {
		return nil, err
	}
	if err := req.Storage.Put(entry); err != nil {
		return nil, err
	}

	return nil, nil
}

type roleEntry struct {
	DBName     string              `json:"db_name" mapstructure:"db_name" structs:"db_name"`
	Statements dbplugin.Statements `json:"statments" mapstructure:"statements" structs:"statments"`
	DefaultTTL time.Duration       `json:"default_ttl" mapstructure:"default_ttl" structs:"default_ttl"`
	MaxTTL     time.Duration       `json:"max_ttl" mapstructure:"max_ttl" structs:"max_ttl"`
}

const pathRoleHelpSyn = `
Manage the roles that can be created with this backend.
`

const pathRoleHelpDesc = `
This path lets you manage the roles that can be created with this backend.

The "sql" parameter customizes the SQL string used to create the role.
This can be a sequence of SQL queries. Some substitution will be done to the
SQL string for certain keys. The names of the variables must be surrounded
by "{{" and "}}" to be replaced.

  * "name" - The random username generated for the DB user.

  * "password" - The random password generated for the DB user.

  * "expiration" - The timestamp when this user will expire.

Example of a decent SQL query to use:

	CREATE ROLE "{{name}}" WITH
	  LOGIN
	  PASSWORD '{{password}}'
	  VALID UNTIL '{{expiration}}';
	GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO "{{name}}";

Note the above user would be able to access everything in schema public.
For more complex GRANT clauses, see the PostgreSQL manual.

The "revocation_sql" parameter customizes the SQL string used to revoke a user.
Example of a decent revocation SQL query to use:

	REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM {{name}};
	REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM {{name}};
	REVOKE USAGE ON SCHEMA public FROM {{name}};
	DROP ROLE IF EXISTS {{name}};
`