import Statistics from './Statistics';
import Overview from './Overview';

// ==============================|| PANEL — ANALYTICS (shadcn) ||============================== //
// Admin marketing/analytics dashboard. shadcn/Tailwind + recharts port of views/Analytics,
// reusing the same `/api/analytics/*` endpoints; only the UI layer is new.

export default function Analytics() {
  return (
    <div className="space-y-6">
      <Statistics />
      <Overview />
    </div>
  );
}
