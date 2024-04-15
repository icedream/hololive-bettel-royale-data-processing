-- name: GetUsers :many
SELECT DISTINCT
    u.`id` AS user_id,
    uno.`name` AS user_name,
    coalesce(
        group_concat(
            olduno.`name`,
            ';'
        ),
        NULL
    ) AS old_user_names
FROM `users` u
LEFT JOIN `user_name_observations` AS uno
    ON uno.user_id = u.`id` AND uno.`id` = (
        SELECT MAX(uno2.`id`)
        FROM `user_name_observations` uno2
        WHERE uno2.`user_id` = u.`id`
    )
LEFT JOIN `user_name_observations` AS olduno
    ON olduno.`user_id` = u.id AND olduno.`id` != uno.`id`
WHERE 
  (uno.`name` = @user_name OR @user_name IS NULL)
  AND (uno.`name` LIKE @user_name_like OR @user_name_like IS NULL)
GROUP BY u.`id`
ORDER BY u.`id` ASC
LIMIT sqlc.narg('max_count')
OFFSET sqlc.narg('offset')
;
