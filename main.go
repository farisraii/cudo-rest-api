package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Organization struct {
	OrgID       string         `json:"org_id"`
	OrgName     string         `json:"org_name"`
	OrgParentID sql.NullString `json:"-"`
	OrgChilds   []Organization `json:"org_childs,omitempty"`
}

func GenerateJSONStructure(orgMap map[string]*Organization, organizationID string) string {
	rootOrg := orgHirarki(orgMap, organizationID)
	jsonData, err := json.Marshal(rootOrg)
	if err != nil {
		log.Fatal("Error marshalling JSON: ", err)
	}
	return string(jsonData)
}

func orgHirarki(orgMap map[string]*Organization, orgID string) Organization {
	org := *orgMap[orgID]
	childOrgs := []Organization{}

	for _, o := range orgMap {
		if o.OrgParentID.Valid && o.OrgParentID.String == orgID {
			childOrgs = append(childOrgs, orgHirarki(orgMap, o.OrgID))
		}
	}

	if len(childOrgs) > 0 {
		org.OrgChilds = childOrgs
	}

	return org
}

func handleRequest(c *gin.Context) {
	organizationID := c.Param("organization_id")
	if organizationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing organization_id parameter"})
		return
	}

	connStr := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error connecting to the database"})
		return
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error pinging database"})
		return
	}

	query := `
		WITH RECURSIVE org_tree AS (
			SELECT org_id, org_name, org_parent_id
			FROM organization
			WHERE org_id = $1 AND org_status = '1'
			UNION ALL
			SELECT o.org_id, o.org_name, o.org_parent_id
			FROM organization o
			INNER JOIN org_tree ot ON o.org_parent_id = ot.org_id
			WHERE o.org_status = '1'
		)
		SELECT * FROM org_tree;
	`
	rows, err := db.Query(query, organizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database"})
		return
	}
	defer rows.Close()

	orgMap := make(map[string]*Organization)

	for rows.Next() {
		var org Organization
		err := rows.Scan(&org.OrgID, &org.OrgName, &org.OrgParentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning row"})
			return
		}
		orgMap[org.OrgID] = &org
	}

	jsonData := GenerateJSONStructure(orgMap, organizationID)
	c.String(http.StatusOK, jsonData)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	router := gin.Default()

	router.POST("/GenerateJSONStructure/:organization_id", handleRequest)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server started on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
