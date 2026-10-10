import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useState } from 'react';
import type { Slot } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { SelectInput, TextInput } from '@/components/FormField';
import { Spinner } from '@/components/Spinner';
import { useDepartments, useDoctors } from '@/features/catalog/api/useCatalog';
import { addDaysYmd, formatDateLong, ymdJst } from '@/lib/format';
import { BookingDialog } from './BookingDialog';
import { SlotPicker } from './SlotPicker';

/** 診療科・医師・日付から空き枠を探し、予約する。 */
export function BookingFlow() {
  const today = ymdJst();
  const [deptId, setDeptId] = useState<number | undefined>();
  const [doctorId, setDoctorId] = useState<number | undefined>();
  const [date, setDate] = useState(today);
  const [picked, setPicked] = useState<Slot | null>(null);

  const departments = useDepartments();
  const doctors = useDoctors(deptId);
  const dept = departments.data?.find((d) => d.id === deptId);

  if (departments.isPending) return <Spinner />;
  if (departments.isError) {
    return (
      <div className="space-y-3">
        <Alert tone="error">{userMessage(parseApiError(departments.error))}</Alert>
        <Button variant="secondary" onClick={() => departments.refetch()}>再試行</Button>
      </div>
    );
  }

  const changeDate = (next: string) => next >= today && setDate(next);

  return (
    <div className="space-y-6">
      <Card className="grid gap-4 sm:grid-cols-2">
        <SelectInput
          label="診療科"
          value={deptId ?? ''}
          onChange={(e) => { setDeptId(e.target.value ? Number(e.target.value) : undefined); setDoctorId(undefined); }}
        >
          <option value="">選択してください</option>
          {departments.data.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
        </SelectInput>
        <SelectInput
          label="医師（任意）"
          disabled={deptId === undefined}
          value={doctorId ?? ''}
          onChange={(e) => setDoctorId(e.target.value ? Number(e.target.value) : undefined)}
        >
          <option value="">指定なし</option>
          {doctors.data?.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
        </SelectInput>
        <div className="sm:col-span-2">
          <div className="flex items-end gap-2">
            <Button variant="secondary" aria-label="前の日" disabled={date <= today} onClick={() => changeDate(addDaysYmd(date, -1))}>
              <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            </Button>
            <div className="flex-1">
              <TextInput label="日付" type="date" min={today} value={date} onChange={(e) => e.target.value && changeDate(e.target.value)} />
            </div>
            <Button variant="secondary" aria-label="次の日" onClick={() => changeDate(addDaysYmd(date, 1))}>
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        </div>
      </Card>

      <Card>
        <h2 className="mb-4 text-lg font-bold">{formatDateLong(`${date}T00:00:00+09:00`)} の空き枠</h2>
        {deptId === undefined ? (
          <p className="py-6 text-center text-neutral-600">診療科を選ぶと、空き枠が表示されます</p>
        ) : (
          <SlotPicker departmentId={deptId} doctorId={doctorId} date={date} doctors={doctors.data ?? []} onSelect={setPicked} />
        )}
      </Card>

      {picked && dept && (
        <BookingDialog
          slot={picked}
          departmentName={dept.name}
          doctorName={doctors.data?.find((d) => d.id === picked.doctor_id)?.name ?? ''}
          onClose={() => setPicked(null)}
        />
      )}
    </div>
  );
}
