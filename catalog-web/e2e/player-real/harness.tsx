// Real-browser harness: renders the REAL MediaPlayer (real api client, real <video>) against the local ticket server.
import { createRoot } from 'react-dom/client'
import { MediaPlayer } from '@/components/media/MediaPlayer'

const id = Number(new URLSearchParams(location.search).get('id') || '1')
;(window as any).playerErrors = 0
createRoot(document.getElementById('r')!).render(
  <MediaPlayer
    media={{ id, title: 'harness', media_type: 'video/mp4', directory_path: 'x', created_at: '', updated_at: '' } as any}
    onError={() => { (window as any).playerErrors++ }}
  />,
)
