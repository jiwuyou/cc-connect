import { useEffect, useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router-dom';
import {
  Server, Heart, ArrowRight, FolderKanban, Plus,
} from 'lucide-react';
import { Card, Badge, Button, Modal, EmptyState } from '@/components/ui';
import {
  listProjects,
  type ProjectSummary,
} from '@/api/projects';
import { restartSystem } from '@/api/status';
import { getAgentLabel } from '@/lib/providers';
import AddProjectWizard from './AddProjectWizard';

export default function ProjectList() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [loading, setLoading] = useState(true);

  const [showWizard, setShowWizard] = useState(false);
  const [showRestartModal, setShowRestartModal] = useState(false);
  const [pendingProjectName, setPendingProjectName] = useState('');

  const fetch = useCallback(async () => {
    try {
      setLoading(true);
      const data = await listProjects();
      setProjects(data.projects || []);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetch();
    const handler = () => fetch();
    window.addEventListener('cc:refresh', handler);
    return () => window.removeEventListener('cc:refresh', handler);
  }, [fetch]);

  const openWizard = () => {
    setShowWizard(true);
  };

  const handleWizardComplete = useCallback(() => {
    setShowWizard(false);
    fetch();
  }, [fetch]);

  const handleWebProjectCreated = useCallback(async (res: { name?: string; restart_required: boolean }) => {
    setPendingProjectName(res.name || '');
    await fetch();
    setShowWizard(false);
    if (res.restart_required) {
      setShowRestartModal(true);
      return;
    }
    if (res.name) {
      navigate(`/projects/${res.name}`);
    }
  }, [fetch, navigate]);

  const waitForService = (maxMs: number) =>
    new Promise<void>((resolve) => {
      const start = Date.now();
      const poll = () => {
        window.fetch('/api/v1/status')
          .then((r) => { if (r.ok) resolve(); else throw new Error(); })
          .catch(() => {
            if (Date.now() - start > maxMs) { resolve(); return; }
            setTimeout(poll, 500);
          });
      };
      setTimeout(poll, 1500);
    });

  if (loading && projects.length === 0) {
    return <div className="flex items-center justify-center h-64 text-gray-400 animate-pulse">Loading...</div>;
  }

  return (
    <div className="animate-fade-in space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <h2 className="text-lg font-bold text-gray-900 dark:text-white">{t('projects.title')}</h2>
        <Button size="sm" onClick={openWizard}>
          <Plus size={14} /> {t('setup.addProject', 'Add project')}
        </Button>
      </div>

      {projects.length === 0 ? (
        <EmptyState message={t('projects.noProjects')} icon={FolderKanban} />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {projects.map((p) => (
            <Link key={p.name} to={`/projects/${p.name}`}>
              <Card hover className="h-full flex flex-col">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <Server size={18} className="text-gray-400" />
                    <h3 className="font-semibold text-gray-900 dark:text-white truncate">{p.display_name || p.name}</h3>
                  </div>
                  <ArrowRight size={16} className="text-gray-300 dark:text-gray-600" />
                </div>
                <div className="flex flex-wrap gap-1.5 mb-3">
                  <Badge variant="info">{getAgentLabel(p.agent_type)}</Badge>
                  {p.platforms?.map((pl) => <Badge key={pl}>{pl}</Badge>)}
                </div>
                <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400 mt-auto pt-3 border-t border-gray-100 dark:border-gray-800">
                  <span>{p.sessions_count} {t('nav.sessions').toLowerCase()}</span>
                  {p.heartbeat_enabled && (
                    <span className="flex items-center gap-1 text-emerald-500"><Heart size={12} /> {t('heartbeat.title')}</span>
                  )}
                </div>
              </Card>
            </Link>
          ))}
        </div>
      )}

      <AddProjectWizard
        open={showWizard}
        onClose={() => setShowWizard(false)}
        onComplete={handleWizardComplete}
        onWebProjectCreated={handleWebProjectCreated}
      />

      <Modal open={showRestartModal} onClose={() => setShowRestartModal(false)} title={t('setup.restartRequired', 'Restart required')}>
        <div className="space-y-4 py-2">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            {t('setup.restartHint', 'Restart the service for the new platform to take effect.')}
          </p>
          <div className="flex justify-end gap-2 flex-wrap">
            <Button variant="secondary" className="max-sm:flex-1" onClick={() => { setShowRestartModal(false); setPendingProjectName(''); fetch(); }}>
              {t('setup.later', 'Later')}
            </Button>
            <Button className="max-sm:flex-1" onClick={async () => {
              await restartSystem();
              await waitForService(8000);
              await fetch();
              setShowRestartModal(false);
              if (pendingProjectName) {
                navigate(`/projects/${pendingProjectName}`);
                setPendingProjectName('');
              }
            }}>
              {t('setup.restartNow', 'Restart now')}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
