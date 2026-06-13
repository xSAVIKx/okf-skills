module okf-mysql

go 1.20

require (
	github.com/go-sql-driver/mysql v1.7.1
	github.com/savikne/okf-skills-registry/okf-go v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/savikne/okf-skills-registry/okf-go => ../../okf-go
