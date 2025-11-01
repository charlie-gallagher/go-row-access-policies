# Row access control program
## Quick start
A simple row access service written in Go.

Uses [go-task](https://github.com/go-task/task) as a build tool.

```sh
task test build
./row_access --db test.db --load config.json
./row_access --db test.db --get pa_sales_manager
## {"role":"pa_sales_manager","policy":[{"column":"Region","values":["Eastern"]},{"column":"State","values":["Pennsylvania"]}]}
```

It's a straightforward service where you write policies for roles, then you can query those policies to see what a particular role has access to. You can load multiple separate config files into the database, which is persisted as a SQLite file. I've been writing this service mainly as a learning tool for Go, but also I guess as a portfolio piece for how I'm thinking about access control (see discussion below).

## Background
In BI applications, programs typically offer row-based controls that limit what
data certain users can view.

At IXIS we have row controls, and I wanted to both think critically about how
we're doing this and at the same time work on my golang skills.
The goal is to create a service that another program can invoke to figure out
what rows of a dataset a user has access to.

The model is a separation of policy and enforcement -- this module will control
the policy, and some other module will control enforcement when it's necessary.
I'm also going to work on the language that these services use to communicate
with each other (e.g. does the policy know what a database column is, or does
it use a higher-level abstraction?).

# Implementation ideas
## Overview of our current system
There is a data database (some data warehouse) that users want to access. This
access is mediated by a query engine that exposes a limited interface to the
user. The query engine has unlimited access to the data warehouse, so it needs a
system (basically a data catalog) that tells it how to map user credentials to
rows in the data. This is done through _control columns_, ie columns in the data
tables that have values that correspond to policies stored in the data catalog.

The data catalog is configured by saying which values in some control column a
user/group has access to. An imaginary policy might look something like this:

```json
{
    "policy": {
        "my_table": {
            "access": [
                {"column": "region", "values": ["Eastern", "Western"]},
                ...
            ]
        }
    }
}
```

Each control column maps directly to a user or group identifier. If it were a
geographically-based system, every dataset that contains geo data would have to
have standard columns like "Region" and "State".

Our implementation maps a policy statement direclty into SQL:

```
{"column": "region", "values": ["Eastern", "Western"]}

WHERE "region" in ('Eastern', 'Western')
```

A typo could mean accidentally leaking sensitive information, so it's critical
that the policy is written carefully and updated when the table is updated.


## Proposed systems

### High-level policies
The policy is stated in terms of a group and its associated values. If the user
is part of the "Eastern" region group, then the policy service interaction might
look like:

```python
uid = 1  # Associated with group Eastern Region
policy = get_user_policy(uid)
print(policy)
## [{"Region": "Eastern"}]
```

Then it's the enforcer's job to take this information and map it to database
objects.

This is a very minimal policy. It's not clear exactly how the query engine would
map the policy `{"Region": "Easter"}` to actual database resources. It would
have to scan the table and look to see if there is a column that maps to the
idea of a "Region", and then generate a SQL filter based on what it finds.


### Low-level policies
This is basically what we have right now. The policy is a deconstructed SQL
statement.

```python
uid = 1
table = "sys_sales"
policy = get_data_policy(uid=uid, table=table)
print(policy)
## [{"column": "sales_region", "values": ["Eastern"]}]
```

It's unambiguous to turn this into SQL, because the person who wrote the policy
basically wrote a SQL condition to apply whenever a user queries the specified
table.

These policies are more sensitive to typos, but they're easy to use and very
fine-grained. Still, it feels delicate to keep a physical mapping of groups to
specific database objects in the policy database itself. If there are many
groups and tables, the size of the policy database increases very quickly, and
each policy is a potential security problem if it gets misconfigured.

Validation is hard, too, because the policy service probably doesn't have access
to the data warehouse itself.

You have to store raw column names in the policy, so the query engine has to be
careful not to rename any columns before applying the filter. But it can apply
the filter wherever it's most effective as long as the column names remain the
same.



### Existing data catalogs
Snowflake has a [row-level
security](https://docs.snowflake.com/en/user-guide/security-row-intro) feature
based on "the use of row access policies to determine which rows to return in
the query result."

Policies are configured based on roles or users and database objects (for role X
ensure policy Y). Here's an example of creating a simple row filter:

```
CREATE OR REPLACE ROW ACCESS POLICY rap_it
AS (empl_id varchar) RETURNS BOOLEAN ->
  'it_admin' = current_role()
;
```

A mapping table maps role/user information (e.g. user's first name) to column
values via a mapping table.


| Sales Manager | Region |
|-------------|---------|
| Alice | WW |
| Bob | NA |
| Simon | EU |


The mapping table is a regular database object, and the policy refers to the
mapping table. In particular, it specifies a subquery against the mapping table.
Skipping over some details, here's an example policy:

```sql
CREATE OR REPLACE ROW ACCESS POLICY security.sales_policy
AS (sales_region varchar) RETURNS BOOLEAN ->
  'sales_executive_role' = CURRENT_ROLE()
    OR EXISTS (
      SELECT 1 FROM salesmanagerregions
        WHERE sales_manager = CURRENT_ROLE()
        AND region = sales_region
    )
;
```

This says that `sales_executive_role` has access to all data, while a
`sales_manager` only has access to data when `salesmanagerregions.region`
is equal to the region column in the data.

There are some Snowflake-specific features here, but there are some points that
relate to the two above implementation ideas that are salient:

- Mapping tables provide a shortcut to creating many, many policies.
- The policy is enforced via a virtual view at query time. The user basically
  queries a modified version of the table.
- Policies have access not only to columns, but to database assets like the role
  name and user name.

I like the idea of creating a virtual view and building the query on top of it,
although I'm not quite at that point in the design process yet. I'm still
thinking about what information to store for policies in the database.

Let's say for discussion that we use Snowflake, or any other service that has
both users and roles. Our application has roles and groups, and the user's
effective policy is the intersection of privileges across all of their roles and
groups. What if we wanted to make use of Snowflake roles and policies? I'd
imagine we could do something like this.

The query engine has a Snowflake User with virtually unlimited read access.
Then, it assumes an appropriate role for the user (which gets looked up in the
policy). It turns out Snowflake _does_ have an idea of secondary roles, and you
can assume multiple roles. So, a regional sales manager could have a main role
of `sales_manager` and a secondary role of `eastern_region`. You would then have
to maintain a mapping of roles to values in the database, but the policy
database would contain only references to roles.

Snowflake is a good example of RBAC, but if we want to maintain independence
from any particular platform, we might need to stick to features that are
broadly available.

### Discussion
Using a service like Snowflake would mean storing two policy sets -- one for
mapping our platform's user accounts to Snowflake roles, and then another for
mapping Snowflake roles to specific row access in a database object. Validation
can be ensured because we can (probably) create the mapping tables
automatically, although I don't know how strict Snowflake is with its validation
of policies. Does it allow you to reference columns that don't exist, and in
that case would it just fail?

I like the idea of failing over quietly ignoring certain policy items. You can
imagine a system that would accidentally ignore a policy. Suppose you stored in
the policy database a mapping from a role and column name to a WHERE statement,
and to enforce that policy you would get all columns in a table and look for
policies associated with them in the policy database.

```py
uid = 1
table = "my_table"
policy = get_policy(uid=uid, table=table, columns=get_columns(table))
```

The policy database would make a query like this:

```sql
select
    policy
from
    policies
where
    uid = 1
    and table = 'my_table'
    and column in (<list of columns>)
```

A typo in the policy would cause the `WHERE x IN y` clause to return nothing
instead of failing.

That's easy to avoid, just get all policies for a table and then apply them. If
there's a typo in how the policy refers to a column name, it'll cause the SQL to
fail (`no column called 'col_with_typo'`).

The mapping between our platform's concept of roles and the warehouse's concept
of roles has to be shaky, there's just no clean way to ensure correctness. This
is the case with all security systems. The best thing to do is validate when you
create the policy and when you update the table that the policy refers to.

All of this is beside the point for this project, though. I want to implement a
basic policy database.

### My basic policy database
I'm going with a high-level design. User roles are mapped to control columns
that have standard names like "State", "Region", etc. The query service
maintains a mapping from the standard names to each actual instance (ie column)
of it.

We already have a service that registers tables with the database, so it
wouldn't be too much work to add a step to the table registration process. In
addition to regular metadata about your table, you would also create a control
column mapping. An over-simplified version would look like this:

```py
register_table_dimensions(
    dimensions=["date", "region", "state", "item_code", "msrp"],
    control_column_map={"Region": "region", "State": "state"}
)
```

This has some nice advantages:

- It's easy to validate that the column names in the `control_column_map` also
  appear in the list of dimensions.
- If you update your phyical table, you also have to update the metadata. This
  makes it much easier to remember to update the `control_column_map` at the
same time.

It's not required that every table have every control column (e.g. some tables
might not have state-level data, only region-level data), so it's the user's
responsibility to create accurate control column mappings with all relevant
keys. The `register_table_dimensions` function can warn when a mapping is
missing potentially important control columns, and the user can use that to
decide if they made a mistake. (When I say user here, I'm referring to the
person creating a new table for the platform.)

So then getting back to the policy service. It only needs to accept as input a
user or role and a list of control columns, and it would return the values that
those control columns may take.

```py
uid = 1
control_columns = ["Region", "State"]
policy = get_policy(uid=uid, control_columns=control_columns)
print(policy)
## {"Region": ["Eastern"], "State": ["Pennsylvania", "New York", ...]}
```

Ok now this seems like it's working well. We could eventually extend it to
return a subquery like Snowflake uses (`{"Region": "select 1 from regions where
uid = get_uid()"}`).

To create a new policy, you just have to map a role to all relevant control
columns and the values of those control columns that they're allowed to see.

```py
create_policy(
    role_name="eastern_sales_manager",
    policy_map={
        "Region": ["Eastern"],
        "State": ["Pennsylvania", "New York", ...]
    }
)
```

The control columns are fundamental to the application, so they're unlikely to
change over time, and the values would likely be standardized so there's low
likelihood of standardization problems. (This is also something you could build
a test for during the development of a new table.)

# Implementation
I'll need:

- A SQLite database (or some embedded database)
- Policy creator functions
- Policy getters

That should be it; very simple. In a real system, I would need to worry about
who can create and modify policies, but here I'm just gonna start with anyone
can read, write, and modify any policy.

## Current state: 2025-10-25
So far, I have a working serde system (read into database, get back from database and back into policy JSON). But there's no user interface. So, I'd like to set up two different UIs, a localhost server and a CLI. The server is better for loading data, although I guess you could pass in a folder containing policies to the CLI.

I guess for a CLI you could set up a persistent database, load it with some files through repeated CLI calls, and then query it. At least that way you don't have to `POST` data, you can just tell the CLI handler about the file you want to load.

Ok, I'm going with a CLI for now. Later, I can put this into a server if I want to experiment with that.

## To-dos
There's not much to this, but there are still things I'd like to improve.

- [ ] Install `config_schema.json` into a standard location (or store natively)
- [ ] 


---

Charlie Gallagher, October 2025
