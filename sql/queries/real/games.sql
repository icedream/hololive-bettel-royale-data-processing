-- name: GetGames :many
SELECT DISTINCT
    `id` AS id,
    `xp_multiplier` AS xp_multiplier,
    `discord_message_id` AS discord_message_id,
    `discord_end_message_id` AS discord_end_message_id,
    `winner_user_id` AS winner_user_id,
    `winner_user_name` AS winner_user_name
FROM `games` g
GROUP BY g.`id`
ORDER BY g.`id` ASC
LIMIT sqlc.narg('max_count')
OFFSET sqlc.narg('offset')
;
