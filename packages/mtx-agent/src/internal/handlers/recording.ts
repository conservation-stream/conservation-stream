import { RecordingRequest } from '@conservation-stream/mtx-manager';
import type { MTXMetadata } from '@conservation-stream/site-api';
import { createReadStream } from 'node:fs';
import { mkdir, stat, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { Readable } from 'node:stream';
import { Queue } from '../queue.ts';

export const handleRecording = async (recording: MTXMetadata['recording'], signal: AbortSignal) => {
  await mkdir(join(recording.directory, 'clips'), { recursive: true });
  console.log(`Connecting to recording queue ${recording.links.queue}`);
  const queue = new Queue(recording.links.queue, { signal });

  for await (const event of queue) {
    console.log('Received message from recording queue');
    const data = JSON.parse(event.data);
    if (data.type !== 'upload') continue;

    const request = RecordingRequest.parse(data.request);
    const params = new URLSearchParams();
    params.set('path', request.params.path);
    params.set('start', request.params.startDate);
    params.set('duration', request.params.duration.toString());

    const stream = await getRecordingFromMediamtx('localhost', recording.playbackAddress, params);
    const clipPath = join(recording.directory, 'clips', `${request.id}.mp4`);
    const { file, size } = await cacheRecordingToDisk(clipPath, stream);

    await uploadFile(request.storage.signedUrl, file, size);
    await completeRecording(request.completeUrl, request.id);
    console.log('Recording completed');
  }
};

const getRecordingFromMediamtx = async (host: string, port: string, params: URLSearchParams) => {
  const url = `http://${host}${port}/get?${params.toString()}`;
  const stream = await fetch(url);
  if (!stream.ok) throw new Error('Failed to get recording stream');
  if (!stream.body) throw new Error('Recording stream is missing body');
  return Readable.fromWeb(stream.body);
};

const completeRecording = async (url: string, id: string) => {
  const response = await fetch(url, { method: 'POST', body: JSON.stringify({ id }), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) throw new Error('Failed to complete recording');
};

const uploadFile = async (url: string, file: Readable, size: number) => {
  const response = await fetch(url, {
    method: 'PUT',
    body: file,
    headers: {
      'Content-Type': 'video/mp4',
      'Content-Length': size.toString()
    }
  });
  if (!response.ok) {
    const message = await response.text();
    throw new Error(`Failed to upload file: ${message}`);
  }
};

const cacheRecordingToDisk = async (path: string, stream: Readable) => {
  await writeFile(path, stream);
  const stats = await stat(path);
  const size = stats.size;
  const file = createReadStream(path);
  return { size, file };
};
