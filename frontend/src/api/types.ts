// openapi-typescript 生成型への別名。型本体は schema.d.ts（生成物・手編集禁止）にある。
// 再生成: npm run gen:api
import type { components } from './schema';

type Schemas = components['schemas'];

export type Department = Schemas['Department'];
export type Doctor = Schemas['Doctor'];
export type Slot = Schemas['Slot'];
export type CreateAppointmentRequest = Schemas['CreateAppointmentRequest'];
export type Appointment = Schemas['Appointment'];
export type AppointmentSummary = Schemas['AppointmentSummary'];
export type WaitTimesResponse = Schemas['WaitTimesResponse'];
export type DepartmentWaitTime = Schemas['DepartmentWaitTime'];
export type Position = Schemas['Position'];
export type WalkInTicket = Schemas['WalkInTicket'];
export type StaffQueueEntry = Schemas['StaffQueueEntry'];
export type ApiError = Schemas['Error'];
