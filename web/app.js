/**
 * GoChat — Frontend Application Logic
 *
 * Responsibilities:
 *  - Authentication guard: redirect to login if no JWT in localStorage
 *  - Fetch and display all rooms in the sidebar
 *  - Create rooms via the modal
 *  - Open a WebSocket connection when a room is selected
 *  - Send and render messages in real-time
 *  - Auto-reconnect on disconnect with exponential backoff
 */

'use strict';

// ── State ─────────────────────────────────────────────────────────────────
const state = {
  token:      localStorage.getItem('token'),
  userID:     Number(localStorage.getItem('userID')),
  username:   localStorage.getItem('username'),
  activeRoom: null,   // { id, name }
  socket:     null,   // active WebSocket instance
  reconnectAttempts: 0,
};

// ── Auth guard ────────────────────────────────────────────────────────────
if (!state.token) window.location.href = '/';

// ── Boot ──────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  // Show the current user's name and avatar initial in the sidebar.
  document.getElementById('username-display').textContent = state.username;
  document.getElementById('user-avatar').textContent = state.username[0].toUpperCase();

  fetchRooms();
});

// ── Auth helpers ──────────────────────────────────────────────────────────

/** Adds the JWT Authorization header to every API request. */
function authHeaders() {
  return { 'Content-Type': 'application/json', 'Authorization': `Bearer ${state.token}` };
}

/** Clears localStorage and redirects to the login page. */
function logout() {
  localStorage.clear();
  window.location.href = '/';
}

// ── Rooms ─────────────────────────────────────────────────────────────────

/** Fetches all rooms from the API and renders them in the sidebar. */
async function fetchRooms() {
  try {
    const res = await fetch('/api/rooms');
    const rooms = await res.json();
    renderRooms(rooms);
  } catch (err) {
    console.error('Failed to fetch rooms:', err);
  }
}

/** Renders an array of room objects into the sidebar list. */
function renderRooms(rooms) {
  const list = document.getElementById('room-list');
  list.innerHTML = '';

  if (!rooms || rooms.length === 0) {
    list.innerHTML = '<li class="room-list-placeholder">No rooms yet — create one!</li>';
    return;
  }

  rooms.forEach(room => {
    const li = document.createElement('li');
    li.className = 'room-item';
    li.textContent = '# ' + room.name;
    li.dataset.id = room.id;
    li.onclick = () => selectRoom(room);
    list.appendChild(li);
  });
}

/** Opens the new-room modal. */
function showNewRoomModal() {
  document.getElementById('new-room-name').value = '';
  document.getElementById('room-error').textContent = '';
  document.getElementById('modal-overlay').classList.remove('hidden');
  setTimeout(() => document.getElementById('new-room-name').focus(), 50);
}

/** Closes the new-room modal. */
function closeModal() {
  document.getElementById('modal-overlay').classList.add('hidden');
}

/** Sends a POST /api/rooms request and refreshes the room list on success. */
async function createRoom() {
  const name = document.getElementById('new-room-name').value.trim();
  if (!name) return;

  try {
    const res = await fetch('/api/rooms', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({ name }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Could not create room');

    closeModal();
    await fetchRooms();
    // Auto-join the newly created room.
    selectRoom(data);
  } catch (err) {
    document.getElementById('room-error').textContent = err.message;
  }
}

// ── Chat ──────────────────────────────────────────────────────────────────

/**
 * Switches the active room: loads message history, then opens a WebSocket.
 * @param {{ id: number, name: string }} room
 */
function selectRoom(room) {
  if (state.activeRoom?.id === room.id) return;

  // Highlight the selected room in the sidebar.
  document.querySelectorAll('.room-item').forEach(el => {
    el.classList.toggle('active', Number(el.dataset.id) === room.id);
  });

  state.activeRoom = room;
  document.getElementById('room-title').textContent = '# ' + room.name;
  document.getElementById('room-status').textContent = 'Loading history…';

  // Clear old messages and load history for the new room.
  clearMessages();
  loadHistory(room.id).then(() => openSocket(room.id));
}

/** Fetches the last 50 messages for a room and renders them. */
async function loadHistory(roomID) {
  try {
    const res = await fetch(`/api/rooms/${roomID}/messages`);
    const messages = await res.json();
    (messages || []).forEach(msg => renderMessage(msg, false));
    scrollToBottom();
  } catch (err) {
    console.error('History fetch failed:', err);
  }
}

/**
 * Opens a WebSocket connection to the selected room.
 * The JWT is passed as a query parameter because browsers don't support
 * custom headers during the WebSocket handshake.
 */
function openSocket(roomID) {
  // Close any previously open socket gracefully.
  if (state.socket) {
    state.socket.onclose = null; // prevent reconnect loop
    state.socket.close();
  }

  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const url = `${proto}://${location.host}/ws/${roomID}?token=${state.token}`;
  const ws = new WebSocket(url);
  state.socket = ws;

  ws.onopen = () => {
    state.reconnectAttempts = 0;
    setConnStatus('online');
    enableInput(true);
    document.getElementById('room-status').textContent = 'Connected';
  };

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    renderMessage(msg, true);
  };

  ws.onclose = () => {
    setConnStatus('offline');
    enableInput(false);
    document.getElementById('room-status').textContent = 'Disconnected — reconnecting…';
    scheduleReconnect(roomID);
  };

  ws.onerror = (err) => {
    console.error('WebSocket error:', err);
  };
}

/**
 * Schedules a reconnect attempt with exponential backoff (max 30s).
 * This prevents hammering the server on transient network issues.
 */
function scheduleReconnect(roomID) {
  if (!state.activeRoom || state.activeRoom.id !== roomID) return;

  const delay = Math.min(1000 * 2 ** state.reconnectAttempts, 30000);
  state.reconnectAttempts++;
  console.log(`Reconnecting in ${delay}ms (attempt ${state.reconnectAttempts})…`);
  setTimeout(() => {
    if (state.activeRoom?.id === roomID) openSocket(roomID);
  }, delay);
}

/**
 * Sends the input field's text to the server via WebSocket.
 * The server echoes it back as a JSON Message, which renderMessage() handles.
 */
function sendMessage() {
  const input = document.getElementById('message-input');
  const text = input.value.trim();
  if (!text || !state.socket || state.socket.readyState !== WebSocket.OPEN) return;

  state.socket.send(text);
  input.value = '';
  input.focus();
}

/** Sends a message when Enter is pressed (Shift+Enter for newline). */
function handleInputKey(event) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault();
    sendMessage();
  }
}

// ── Render helpers ────────────────────────────────────────────────────────

/**
 * Renders a single message bubble into the messages area.
 * @param {object} msg  - Message object from the API or WebSocket
 * @param {boolean} animate - Whether to animate the bubble in
 */
function renderMessage(msg, animate) {
  // Hide the empty-state placeholder.
  document.getElementById('messages-placeholder')?.remove();

  const isOwn = msg.user_id === state.userID;
  const container = document.createElement('div');
  container.className = 'msg ' + (isOwn ? 'own' : 'other');

  const meta = document.createElement('div');
  meta.className = 'msg-meta';
  meta.textContent = isOwn ? 'You' : msg.username;

  const bubble = document.createElement('div');
  bubble.className = 'msg-bubble';
  bubble.textContent = msg.content;

  container.appendChild(meta);
  container.appendChild(bubble);

  const area = document.getElementById('messages-area');
  area.appendChild(container);

  // Only scroll to bottom for new messages, not history.
  if (animate) scrollToBottom();
}

/** Clears all messages from the messages area and shows the placeholder. */
function clearMessages() {
  const area = document.getElementById('messages-area');
  area.innerHTML = `
    <div class="messages-placeholder" id="messages-placeholder">
      <div class="placeholder-icon">💬</div>
      <p>Loading messages…</p>
    </div>`;
}

/** Scrolls the messages area to the very bottom. */
function scrollToBottom() {
  const area = document.getElementById('messages-area');
  area.scrollTop = area.scrollHeight;
}

/** Enables or disables the message input and send button. */
function enableInput(enabled) {
  document.getElementById('message-input').disabled = !enabled;
  document.getElementById('send-btn').disabled = !enabled;
  if (enabled) document.getElementById('message-input').focus();
}

/** Updates the connection status indicator dot in the header. */
function setConnStatus(status) {
  const el = document.getElementById('conn-indicator');
  el.className = 'conn-indicator ' + status;
  el.title = status === 'online' ? 'Connected' : 'Disconnected';
}
