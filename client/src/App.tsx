import React, { useState, useCallback, useEffect, useRef } from 'react';

const App = () => {
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [taskId, setTaskId] = useState<number | null>(null);
  const [status, setStatus] = useState<string>('Готов к загрузке');
  const [progress, setProgress] = useState(0);
  const [isDragActive, setIsDragActive] = useState(false);
  const [selectedAction, setSelectedAction] = useState<string | null>(null);
  const [resultMessage, setResultMessage] = useState<string | null>(null);
  const ws = useRef<WebSocket | null>(null);

  const handleDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setIsDragActive(true);
    } else if (e.type === 'dragleave') {
      setIsDragActive(false);
    }
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      setFile(e.dataTransfer.files[0]);
    }
  }, []);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      setFile(e.target.files[0]);
    }
  };

  const CHUNK_SIZE = 5 * 1024 * 1024;

  const uploadFile = async () => {
    if (!file || !selectedAction) return;
    setUploading(true);
    setStatus('Инициализация загрузки...');
    setProgress(0);
    setResultMessage(null);

    try {
      const initFormData = new FormData();
      initFormData.append('file_name', file.name);

      const initResponse = await fetch('/api/v1/upload', {
        method: 'POST',
        body: initFormData,
      });

      if (!initResponse.ok) {
        const text = await initResponse.text();
        throw new Error(`Ошибка инициализации (${initResponse.status}): ${text || 'пустой ответ'}`);
      }

      const initData = await initResponse.json();
      const { upload_id, task_id } = initData;
      setTaskId(task_id);
      console.log('Upload ID:', upload_id, 'Task ID:', task_id);

      const totalParts = Math.ceil(file.size / CHUNK_SIZE);
      const completedParts: { e_tag: string; part_number: number }[] = [];

      for (let i = 1; i <= totalParts; i++) {
        const start = (i - 1) * CHUNK_SIZE;
        const end = Math.min(start + CHUNK_SIZE, file.size);
        const chunk = file.slice(start, end);

        const chunkFormData = new FormData();
        chunkFormData.append('file', chunk);
        chunkFormData.append('file_name', file.name);
        chunkFormData.append('file_size', chunk.size.toString());
        chunkFormData.append('upload_id', upload_id);
        chunkFormData.append('part_number', i.toString());

        setStatus(`Загрузка части ${i} из ${totalParts}...`);

        const chunkResponse = await fetch('/api/v1/upload/continue', {
          method: 'POST',
          body: chunkFormData,
        });

        if (!chunkResponse.ok) {
          const text = await chunkResponse.text();
          throw new Error(`Ошибка части ${i} (${chunkResponse.status}): ${text || 'пустой ответ'}`);
        }

        const chunkData = await chunkResponse.json();
        completedParts.push({ e_tag: chunkData.e_tag, part_number: i });
        setProgress(Math.round((i / totalParts) * 100));
      }

      setStatus('Финализация загрузки...');
      const finishResponse = await fetch('/api/v1/upload/finish', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          upload_id,
          task_id,
          file_name: file.name,
          parts: completedParts,
          action: selectedAction,
          size: file.size
        }),
      });

      if (!finishResponse.ok) {
        const text = await finishResponse.text();
        throw new Error(`Ошибка финализации (${finishResponse.status}): ${text || 'пустой ответ'}`);
      }

      setStatus('Загрузка завершена! Обработка...');
      connectWebSocket(task_id);
    } catch (err: any) {
      console.error('Ошибка загрузки:', err);
      setStatus(`Ошибка: ${err.message || 'Неизвестная ошибка'}`);
      setUploading(false);
    }
  };

  const connectWebSocket = (id: number) => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws?task_id=${id}`;
    console.log('Подключение к WebSocket:', wsUrl);
    const socket = new WebSocket(wsUrl);

    socket.onopen = () => {
      console.log('WebSocket соединение установлено');
    };

    socket.onmessage = (event) => {
      console.log('Получено сообщение от WS:', event.data);
      const message = event.data;

      const progressMatch = message.match(/(\d+(?:\.\d+)?)%/);
      if (progressMatch) {
        const val = parseFloat(progressMatch[1]);
        if (!isNaN(val)) setProgress(val);
      }

      if (message.includes('готово')) {
        setStatus('Ваше видео готово');
        setResultMessage(message);
        setUploading(false);
        setProgress(100);
        socket.close();
      } else if (message.includes('ошибка')) {
        setStatus('Ошибка обработки');
        setResultMessage(message);
        setUploading(false);
        socket.close();
      } else {
        setStatus(message);
      }
    };

    socket.onerror = (err) => {
      console.error('Ошибка WebSocket:', err);
      setStatus('Ошибка соединения WebSocket');
    };

    socket.onclose = () => {
      console.log('WebSocket соединение закрыто');
    };

    ws.current = socket;
  };

  useEffect(() => {
    return () => {
      if (ws.current) ws.current.close();
    };
  }, []);

  return (
    <div className="container">
      <header className="hero">
        <h1>Облачный Конвертер</h1>
        <p>Безопасная и быстрая обработка медиа прямо в вашем браузере.</p>
      </header>

      <main>
        <div className="upload-card">
          <div
            className={`drop-zone ${isDragActive ? 'active' : ''}`}
            onDragEnter={handleDrag}
            onDragLeave={handleDrag}
            onDragOver={handleDrag}
            onDrop={handleDrop}
            onClick={() => document.getElementById('file-input')?.click()}
          >
            <input
              id="file-input"
              type="file"
              className="file-input"
              onChange={handleFileChange}
            />
            {file ? (
              <p>{file.name}</p>
            ) : (
              <p>Перетащите файл или нажмите для выбора</p>
            )}
          </div>

          <div className="action-selector">
            <div
              className={`action-option ${selectedAction === 'compress' ? 'active' : ''}`}
              onClick={() => setSelectedAction('compress')}
            >
              <h3>Сжать</h3>
              <p>Уменьшить размер файла</p>
            </div>
            <div
              className={`action-option ${selectedAction === 'mp4' ? 'active' : ''}`}
              onClick={() => setSelectedAction('mp4')}
            >
              <h3>В MP4</h3>
              <p>Конвертировать в MP4</p>
            </div>
          </div>

          <button
            className="upload-btn"
            disabled={!file || uploading || !selectedAction}
            onClick={uploadFile}
          >
            {uploading ? 'Обработка...' : 'Загрузить файл'}
          </button>

          {taskId && (
            <div className="status-container">
              <p className="task-id">ID Задачи: {taskId}</p>
              <div className="progress-bar">
                <div
                  className="progress-fill"
                  style={{ width: `${progress}%` }}
                ></div>
              </div>
              <p className="status-text">{status}</p>

              {resultMessage && (
                <div className="result-area">
                  <h4>Результат:</h4>
                  <p>
                    {(() => {
                      const urlMatch = resultMessage.match(/https?:\/\/[^\s]+/);
                      if (urlMatch) {
                        const url = urlMatch[0];
                        const parts = resultMessage.split('по этой ссылке:');
                        return (
                          <>
                            {parts[0]}
                            <a href={url} target="_blank" rel="noopener noreferrer" className="result-link">
                              по этой ссылке
                            </a>
                            {parts[1] ? parts[1].replace(url, '') : ''}
                          </>
                        );
                      }
                      return resultMessage;
                    })()}
                  </p>
                </div>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
};

export default App;
