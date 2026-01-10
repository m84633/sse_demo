-- name: CreateNotification :execresult
INSERT INTO notifications (room, type, title, body) VALUES (?, ?, ?, ?);

-- name: ListNotificationsByRoom :many
SELECT id, room, type, title, body, created_at
FROM notifications
WHERE room = ?
ORDER BY created_at DESC
LIMIT ?;
