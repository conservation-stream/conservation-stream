export class DocumentNotFoundError extends Error {
  name = 'DocumentNotFoundError';
  constructor(message: string) {
    super(message);
  }
}

export class ProcessingError extends Error {
  name = 'ProcessingError';
  constructor(message: string) {
    super(message);
  }
}

export class FetchError extends Error {
  name = 'FetchError';
  status: number;
  body: unknown;
  constructor(message: string, status: number, body: unknown) {
    super(message);
    this.status = status;
    this.body = body;
  }
}
