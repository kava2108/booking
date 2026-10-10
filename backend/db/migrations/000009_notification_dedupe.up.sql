-- IMP 14-clinic-reservation-notification-outbox TASK-0001
-- リマインドと「あと N 人」通知は 1 件につき 1 回だけ。登録側が並行して動いても二重に登録されないよう、
-- DB の部分ユニークインデックスで保証する（INSERT ... ON CONFLICT DO NOTHING で冪等に登録する）。
-- 確認/取消/変更の通知は、同じ予約に対して複数あり得るため対象外。
CREATE UNIQUE INDEX notifications_reminder_once_key
    ON notifications ((payload ->> 'appointment_id'))
    WHERE kind = 'appointment_reminder';

CREATE UNIQUE INDEX notifications_queue_near_once_key
    ON notifications ((payload ->> 'queue_entry_id'))
    WHERE kind = 'queue_near';
