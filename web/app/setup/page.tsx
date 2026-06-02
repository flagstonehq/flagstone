import { SetupForm } from "@/components/setup/setup-form";

export default function SetupPage() {
  return (
    <main className="min-h-screen flex items-center justify-center bg-bg">
      <div className="w-full max-w-sm">
        <div className="bg-surface rounded-xl border border-border p-8 shadow-sm">
          <div className="mb-6 text-center">
            <div className="inline-flex items-center justify-center w-full mb-2">
              <span className="text-2xl font-bold text-primary">⚑ Flagstone</span>
            </div>
            <p className="text-sm text-text-secondary">
              Set up your feature flag instance
            </p>
          </div>
          <SetupForm />
        </div>
      </div>
    </main>
  );
}
