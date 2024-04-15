-- name: GetItems :many
SELECT DISTINCT
    COALESCE(`name`, '') AS name
FROM `items` item
GROUP BY item.`name`
ORDER BY item.`name` ASC
LIMIT sqlc.narg('max_count')
OFFSET sqlc.narg('offset')
;
