let gws = null;
const { encode, decode } = MessagePack;

const joinMessage = 1;
const broadcastMessage = 2;
const customMessage = 3;

const onAuthenticationFailedMessage = 0x11;
const onJoinFailedMessage = 0x12;
const onJoinMessage = 0x13;
const onOtherUserJoinMessage = 0x14;
const onOtherUserLeaveMessage = 0x15;
const onBroadcastMessage = 0x16;

const statusElem = document.getElementById('status-text');
const userListElem = document.getElementById('userlist-inner');

joinButton.addEventListener('click', () => {
    if (gws !== null) {
        gws.close();
        gws = null;
    }

    const room = roomInput.value;
    const name = nameInput.value;
    if (room.length === 0 || name.length > 8) {
        alert('inalid room or name');
        return;
    }

    const userMap = new Map();
    const memo = new Map();

    const ws = new WebSocket(`ws://${document.location.host}/ws`);
    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
        statusElem.innerText = 'Connected';
        gws = ws;

        ws.send(encode({
            kind: joinMessage,
            body: {
                userId: name,
                secret: '',
                roomId: room,
            }
        }));
    };

    ws.onmessage = e => {
        const msg = decode(e.data);
        const body = msg.body;
        switch (msg.kind) {
            case onAuthenticationFailedMessage:
                alert('invalid name');
                break;
            case onJoinFailedMessage:
                break;
            case onJoinMessage:
                Object.entries(body.users).forEach(([k, v]) => {
                    userMap.set(v.index, v);
                });
                updateUserList(userMap);
                break;
            case onOtherUserJoinMessage:
                userMap.set(body.user.index, body.user);
                updateUserList(userMap);
                break;
            case onOtherUserLeaveMessage:
                memo.delete(body.user.index);
                userMap.delete(body.user.index);
                updateUserList(userMap);
                break;
            case onBroadcastMessage:
                handleBradcast(memo, body.from, decode(body.body));
                break;
            default:
                break;
        }
        console.log(msg);
    };

    ws.onclose = () => {
        statusElem.innerText = 'NotConnected';
        if (gws === ws) {
            gws = null;
        }
    };
});

canvas.addEventListener('pointerdown', onPointerDown);
canvas.addEventListener('pointermove', onPointerMove);
canvas.addEventListener('pointerup', onEnd);
const ctx = canvas.getContext('2d');
ctx.lineWidth = 10;
ctx.lineJoin = 'round';
ctx.lineCap = 'round';

let isDrawing = true;

function onPointerDown(e) {
    if (gws === null) return;
    isDrawing = true;
    gws.send(encode({
        kind: broadcastMessage,
        body: encode({
            k: 0,
            p: getPos(e),
        }),
    }));
}

function onPointerMove(e) {
    if (!isDrawing || gws === null) return;
    gws.send(encode({
        kind: broadcastMessage,
        body: encode({
            k: 1,
            p: getPos(e),
        }),
    }));
}

function onEnd(e) {
    if (!isDrawing) return;
    isDrawing = false;
    if (gws === null) return;
    gws.send(encode({
        kind: broadcastMessage,
        body: encode({
            k: 2,
        }),
    }));
}

clearButton.addEventListener('click', clear);

function clear() {
    if (gws === null) return;
    gws.send(encode({
        kind: broadcastMessage,
        body: encode({
            k: 100,
        }),
    }));
}

function getPos(e) {
    const rect = this.canvas.getBoundingClientRect();
    return {
        x: (e.clientX - rect.left),
        y: (e.clientY - rect.top),
    };
}

function handleBradcast(memo, user, obj) {
    if (obj.k === 100) {
        ctx.fillStyle = 'white';
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        ctx.fillStyle = 'black';
        return;
    }

    let prevPos = memo.get(user.index);
    const p = obj.p;
    if (prevPos == null) {
        if (obj.k === 0) {
            memo.set(user.index, p);
            ctx.beginPath();
            ctx.arc(p.x, p.y, 5, 0, Math.PI * 2);
            ctx.fill();
        }
    }
    else {
        if (obj.k === 0) {
            memo.delete(user.index);
        }
        else if (obj.k === 1) {
            memo.set(user.index, p);

            ctx.beginPath();
            ctx.moveTo(prevPos.x, prevPos.y);
            ctx.lineTo(p.x, p.y);
            ctx.stroke();
        }
        else if (obj.k === 2) {
            memo.delete(user.index);
        }
    }
    console.log(obj);
}

function updateUserList(map) {
    userListElem.innerText = '';
    map.forEach(user => {
        const d = document.createElement('div');
        d.innerText = user.name;
        userListElem.appendChild(d);
    });
}