
import os
import io
import unittest
import time
import threading
import subprocess
import signal
import base64

import requests
import websocket
#websocket.enableTrace(True)

TEST_APP_ID = 'dummy-app-id-123'
TEST_USER_ID = 'dummy-user-id-123'
TEST_HOST = 'localhost'
TEST_PORT = 8080

def generate_handshake_token(user_id, app_id):
    plain_text = '{"user_id": "%s", "app_id": "%s"}' % (user_id, app_id)
    tkn = base64.b64encode(plain_text.encode('utf-8'))
    return tkn.decode('unicode_escape')

def make_ws_url(stream_type, app_id=TEST_APP_ID, user_id=TEST_USER_ID):
    host = TEST_HOST
    port = TEST_PORT
    token = generate_handshake_token(user_id, app_id)
    return 'ws://{host}:{port}/v1/streams/?handshake_token={token}' \
           '&app_id={app_id}&type={stream_type}'.format(host=host, port=port,
           token=token, app_id=app_id, stream_type=stream_type)

def initiate_ws_request(stream_type, app_id=TEST_APP_ID, user_id=TEST_USER_ID,
                        handshake_token=None):
    # Return the response from the request
    if handshake_token == None:
        handshake_token = generate_handshake_token(user_id, app_id)
    host = TEST_HOST
    port = TEST_PORT
    headers = {
        'Upgrade': 'websocket',
        'Connection': 'Upgrade',
        'Sec-Websocket-Key': 'Key-Placeholder-123',
        'Sec-Websocket-Version': 13
    }
    payload = {
        'handshake_token': handshake_token,
        'app_id': app_id,
        'user_id': user_id
    }
    url = 'http://{host}:{port}/v1/streams/?handshake_token={token}' \
           '&app_id={app_id}&type={stream_type}'.format(host=host, port=port,
           token=handshake_token, app_id=app_id, stream_type=stream_type)
    r = requests.get(url, params=payload, headers=headers)
    return r

class StreamerTest(unittest.TestCase):
    def setUp(self):
        env = dict(os.environ, SIPHON_ENV="testing")
        self._proc = subprocess.Popen(
            ['./streamer'],
            env=env,
            cwd='../',
            preexec_fn=os.setsid, # process group
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
        # Wait until it has started up
        try:
            with io.TextIOWrapper(self._proc.stderr, encoding='utf8') as reader:
                reader.readline()
                time.sleep(0.5)
                if self._proc.poll() == 1: # check it hasn't exited
                    raise RuntimeError('Failed to start the streamer: %s' %
                        reader.read())
        except KeyboardInterrupt:
            os.killpg(self._proc.pid, signal.SIGTERM) # kill the whole group
            raise

    def tearDown(self):
        os.killpg(self._proc.pid, signal.SIGTERM) # kill the whole group
        self._proc.wait()

class TestAuth(StreamerTest):
    def test_invalid_handshake_token(self):
        response = initiate_ws_request('notifications',
                                        handshake_token='bad-tkn')
        self.assertEqual(response.status_code, 400)

    def test_valid_handshake_token_valid_url(self):
        response = initiate_ws_request('notifications')
        upgrade = response.headers.get('Upgrade')
        self.assertEqual(upgrade, 'websocket')

class TestLogs(StreamerTest):

    def test_reader_writer(self):
        writer_ws = websocket.create_connection(make_ws_url('log_writer'))
        reader_ws = websocket.create_connection(make_ws_url('log_reader'))

        # Write some log messages
        messages = ['log line 1', 'log line 2', '[ERROR] some error!']
        for message in messages:
            writer_ws.send(message)

        # Read until nothing more can be read
        received = []
        try:
            for i in range(len(messages)):
                received.append(reader_ws.recv())
        except websocket.WebSocketTimeoutException:
            pass

        # Close both sockets
        writer_ws.close()
        reader_ws.close()

        # The messages should match
        self.assertListEqual(messages, received)

    def test_read_with_no_writers(self):
        reader_ws = websocket.create_connection(make_ws_url('log_reader'),
            timeout=0.2)
        with self.assertRaises(websocket.WebSocketTimeoutException):
            reader_ws.recv()
        reader_ws.close()

    def test_write_long_line(self):
        writer_ws = websocket.create_connection(make_ws_url('log_writer'))
        writer_ws.send('abc-123' * 1024 * 100)
        self.assertEqual(writer_ws.recv(), '')
        writer_ws.close()
