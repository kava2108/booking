// openapi-typescript の生成型（schema.d.ts・手編集禁止）への別名。API の型を手書きしない。
// 再生成: npm run gen:api
import type { components, paths } from './schema';

type S = components['schemas'];

export type Department = S['Department'];
export type Doctor = S['Doctor'];
export type Slot = S['Slot'];
export type CreateAppointmentRequest = S['CreateAppointmentRequest'];
export type Appointment = S['Appointment'];
export type AppointmentSummary = S['AppointmentSummary'];
export type WaitTimesResponse = S['WaitTimesResponse'];
export type DepartmentWaitTime = S['DepartmentWaitTime'];
export type Position = S['Position'];
export type WalkInTicket = S['WalkInTicket'];
export type QueueState = S['QueueState'];
export type StaffQueueEntry = S['StaffQueueEntry'];
export type StaffAppointment = S['StaffAppointment'];
export type ClosureRequest = S['ClosureRequest'];
export type ClosureResult = S['ClosureResult'];
export type Settings = S['Settings'];
export type SettingsRequest = S['SettingsRequest'];
export type DepartmentSettings = S['DepartmentSettings'];
export type ScheduleItem = S['ScheduleItem'];
export type Schedules = S['Schedules'];
export type SchedulesRequest = S['SchedulesRequest'];
export type RegisterRequest = S['RegisterRequest'];
export type RegisterResponse = S['RegisterResponse'];
export type LoginRequest = S['LoginRequest'];
export type TokenResponse = S['TokenResponse'];
export type MessageResponse = S['MessageResponse'];
export type PasswordResetRequest = S['PasswordResetRequest'];
export type PasswordResetConfirmRequest = S['PasswordResetConfirmRequest'];
export type ApiErrorBody = S['Error'];
export type ValidationErrorBody = S['ValidationError'];

/** users.role の値（backend/internal/domain/role.go と一致）。 */
export type Role = 'patient' | 'staff' | 'clinic_admin' | 'system_admin';

export type { paths };
