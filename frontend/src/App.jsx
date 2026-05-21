import React, { useState, useEffect, useRef } from 'react';
import './index.css';

const nowPlusHours = (hours) => {
    const d = new Date();
    d.setHours(d.getHours() + hours);
    const pad = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

export default function App() {
    const [output, setOutput] = useState("");
    const [usersCache, setUsersCache] = useState([]);
    const [parkingsCache, setParkingsCache] = useState([]);
    const [bookingsCache, setBookingsCache] = useState([]);

    const request = async (path, method = 'GET', body = null) => {
        try {
            const res = await fetch(path, {
                method,
                headers: body ? { 'Content-Type': 'application/json' } : undefined,
                body: body ? JSON.stringify(body) : undefined
            });

            const text = await res.text();
            let data = text;
            try { data = text ? JSON.parse(text) : null; } catch (_) {}

            const resultObj = { path, status: res.status, ok: res.ok, data };
            setOutput(JSON.stringify(resultObj, null, 2));

            if (!res.ok) {
                const message = (data && typeof data === 'object' && (data.error || data.message)) ||
                    (typeof data === 'string' && data.trim()) ||
                    `Request failed with status ${res.status}`;
                alert(message);
            }
            return { res, data };
        } catch (e) {
            setOutput(JSON.stringify({ error: String(e) }, null, 2));
            return { res: { ok: false }, data: null };
        }
    };

    const normalizeItems = (data) => {
        if (Array.isArray(data)) return data;
        if (Array.isArray(data?.items)) return data.items;
        if (Array.isArray(data?.data)) return data.data;
        return [];
    };

    const loadUsers = async () => {
        const { data } = await request('/api/users');
        setUsersCache(normalizeItems(data));
    };

    const loadParkings = async () => {
        const { data } = await request('/api/parking/parkings');
        setParkingsCache(normalizeItems(data));
    };

    const loadBookings = async () => {
        const { data } = await request('/api/bookings');
        setBookingsCache(normalizeItems(data));
    };

    useEffect(() => {
        loadUsers();
        loadParkings();
        loadBookings();
    }, []);

    const HealthCard = () => (
        <div className="card">
            <h2>Gateway & Service Health</h2>
            <div className="stack">
                <button onClick={() => request('/health')}>Ping Gateway</button>
                <div className="row">
                    <button className="secondary" onClick={() => request('/api/auth/health')}>Auth</button>
                    <button className="secondary" onClick={() => request('/api/users/health')}>Users</button>
                </div>
                <div className="row" style={{marginTop: '0px'}}>
                    <button className="secondary" onClick={() => request('/api/parking/health')}>Parking</button>
                    <button className="secondary" onClick={() => request('/api/bookings/health')}>Booking</button>
                </div>
                <div className="row" style={{marginTop: '0px'}}>
                    <button className="secondary" onClick={() => request('/api/payments/health')}>Payment</button>
                    <button className="secondary" onClick={() => request('/api/notifications/health')}>Notif</button>
                </div>
            </div>
        </div>
    );

    const AuthCard = () => {
        const refs = { regEmail: useRef(), regPass: useRef(), regFirst: useRef(), regLast: useRef(), logEmail: useRef(), logPass: useRef() };
        return (
            <div className="card">
                <h2>Authentication</h2>
                <div className="stack">
                    <label>Login</label>
                    <input ref={refs.logEmail} placeholder="user@mail.com" />
                    <input ref={refs.logPass} type="password" placeholder="password" />
                    <button onClick={() => request('/api/auth/login', 'POST', { email: refs.logEmail.current.value, password: refs.logPass.current.value })}>Login</button>

                    <label style={{marginTop: '16px'}}>Register</label>
                    <input ref={refs.regEmail} placeholder="Email" />
                    <input ref={refs.regPass} type="password" placeholder="Password" />
                    <div className="row">
                        <input ref={refs.regFirst} placeholder="First name" />
                        <input ref={refs.regLast} placeholder="Last name" />
                    </div>
                    <button className="secondary" onClick={async () => {
                        const res = await request('/api/auth/register', 'POST', { email: refs.regEmail.current.value, password: refs.regPass.current.value, first_name: refs.regFirst.current.value, last_name: refs.regLast.current.value });
                        if (res.res.ok) loadUsers();
                    }}>Register</button>
                </div>
            </div>
        );
    };

    const AdminUserCard = () => {
        const refs = { email: useRef(), first: useRef(), last: useRef(), role: useRef(), banId: useRef(), banReason: useRef() };
        return (
            <div className="card">
                <h2>User Management (Admin)</h2>
                <label>Create User</label>
                <input ref={refs.email} placeholder="user@mail.com" />
                <div className="row">
                    <input ref={refs.first} placeholder="First Name" />
                    <input ref={refs.last} placeholder="Last Name" />
                </div>
                <select ref={refs.role} style={{marginTop: '10px'}}><option value="user">User</option><option value="admin">Admin</option></select>
                <button onClick={async () => {
                    if ((await request('/api/users', 'POST', { email: refs.email.current.value, first_name: refs.first.current.value, last_name: refs.last.current.value, role: refs.role.current.value })).res.ok) loadUsers();
                }}>Create User</button>

                <label style={{marginTop: '16px'}}>Ban User</label>
                <input ref={refs.banId} list="users_list" placeholder="User ID" />
                <input ref={refs.banReason} placeholder="Reason" style={{marginTop: '10px'}} />
                <button className="secondary" onClick={() => request(`/api/users/${refs.banId.current.value}/ban`, 'POST', { reason: refs.banReason.current.value })}>Ban User</button>
            </div>
        );
    };

    const ParkingCard = () => {
        const refs = { name: useRef(), address: useRef(), spots: useRef(), pid: useRef() };
        return (
            <div className="card">
                <h2>Parking Management</h2>
                <label>Create Parking</label>
                <input ref={refs.name} placeholder="Parking Name" />
                <input ref={refs.address} placeholder="Address" style={{marginTop: '10px'}}/>
                <label>Capacity (Total Spots limit)</label>
                <input ref={refs.spots} type="number" defaultValue="10" />
                <button onClick={async () => {
                    if ((await request('/api/parking/parkings', 'POST', { name: refs.name.current.value, address: refs.address.current.value, total_spots: Number(refs.spots.current.value) })).res.ok) loadParkings();
                }}>Create Parking</button>
                <button className="secondary" onClick={loadParkings}>List Parkings</button>

                <label style={{marginTop: '16px'}}>Get Parking by ID</label>
                <input ref={refs.pid} list="parkings_list" placeholder="Parking ID" />
                <button className="secondary" onClick={() => request(`/api/parking/parkings/${refs.pid.current.value}`)}>Get Parking Info</button>
            </div>
        );
    };

    const SpotsCard = () => {
        const refs = { pId: useRef(), spotNum: useRef(), actionId: useRef(), status: useRef() };
        return (
            <div className="card">
                <h2>Spots & Tariff</h2>
                <label>Create Spot</label>
                <input ref={refs.pId} list="parkings_list" placeholder="Parking ID" />
                <input ref={refs.spotNum} placeholder="Spot Number (e.g. A1)" style={{marginTop: '10px'}} />
                <button onClick={() => request('/api/parking/spots', 'POST', { parking_id: Number(refs.pId.current.value), number: refs.spotNum.current.value })}>Create Spot</button>
                <button className="secondary" onClick={() => request(`/api/parking/parkings/${refs.pId.current.value}/spots`)}>List Spots by Parking ID</button>

                <label style={{marginTop: '16px'}}>Spot Actions</label>
                <input ref={refs.actionId} placeholder="Spot ID" />
                <div className="row" style={{marginTop: '10px'}}>
                    <button className="secondary" onClick={() => request(`/api/parking/spots/${refs.actionId.current.value}/reserve`, 'POST')}>Reserve</button>
                    <button className="secondary" onClick={() => request(`/api/parking/spots/${refs.actionId.current.value}/release`, 'POST')}>Release</button>
                </div>

                <label style={{marginTop: '16px'}}>Tariff Calculator</label>
                <button className="secondary" onClick={() => request(`/api/parking/tariffs/${refs.pId.current.value}/calculate?hours=2`)}>Test Tariff (2 Hours)</button>
            </div>
        );
    };

    const BookingCard = () => {
        const refs = { uId: useRef(), pId: useRef(), plate: useRef(), start: useRef(), end: useRef(), spotId: useRef(), hours: useRef(), actionId: useRef() };
        useEffect(() => { if(refs.start.current && refs.end.current) { refs.start.current.value = nowPlusHours(1); refs.end.current.value = nowPlusHours(2); } }, []);

        return (
            <div className="card">
                <h2>Booking Operations</h2>
                <label>Create New Booking</label>
                <input ref={refs.uId} list="users_list" placeholder="User ID" />
                <input ref={refs.pId} list="parkings_list" placeholder="Parking ID" style={{marginTop: '10px'}} />
                <input ref={refs.plate} placeholder="Vehicle Plate" style={{marginTop: '10px'}} />
                <div className="row" style={{marginTop: '10px'}}>
                    <div><label>Start</label><input ref={refs.start} type="datetime-local" /></div>
                    <div><label>End</label><input ref={refs.end} type="datetime-local" /></div>
                </div>
                <input ref={refs.spotId} type="number" placeholder="Spot ID (Leave empty to auto-assign)" style={{marginTop: '10px'}} />
                <button onClick={async () => {
                    const payload = { user_id: refs.uId.current.value, parking_id: Number(refs.pId.current.value), vehicle_plate: refs.plate.current.value, start_time: new Date(refs.start.current.value).toISOString(), end_time: new Date(refs.end.current.value).toISOString() };
                    if (refs.spotId.current.value) payload.spot_id = Number(refs.spotId.current.value);
                    if ((await request('/api/bookings', 'POST', payload)).res.ok) loadBookings();
                }}>Create Booking</button>

                <label style={{marginTop: '16px'}}>Booking Actions</label>
                <input ref={refs.actionId} list="bookings_list" placeholder="Booking ID" />
                <div className="row" style={{marginTop: '10px'}}>
                    <button className="secondary" onClick={async () => { if ((await request(`/api/bookings/${refs.actionId.current.value}/confirm`, 'POST')).res.ok) loadBookings(); }}>Confirm</button>
                    <button className="secondary" onClick={async () => { if ((await request(`/api/bookings/${refs.actionId.current.value}/start`, 'POST')).res.ok) loadBookings(); }}>Start</button>
                    <button className="secondary" onClick={async () => { if ((await request(`/api/bookings/${refs.actionId.current.value}/complete`, 'POST')).res.ok) loadBookings(); }}>Complete</button>
                </div>
                <button className="secondary" onClick={async () => { if ((await request(`/api/bookings/${refs.actionId.current.value}/cancel`, 'POST', {reason: 'Frontend cancel'})).res.ok) loadBookings(); }}>Cancel Booking</button>
                <button className="secondary" onClick={loadBookings} style={{marginTop: '10px'}}>Refresh Bookings List</button>
            </div>
        );
    };

    const PaymentNotifCard = () => {
        const refs = {
            payBkId: useRef(), payMethod: useRef(),
            notifUid: useRef(), notifTitle: useRef(),
            emailTo: useRef(), emailSubject: useRef(), emailBody: useRef(), emailUserId: useRef(),
        };
        return (
            <div className="card">
                <h2>Payments & Notifications</h2>

                <label>Process Payment</label>
                <input ref={refs.payBkId} list="bookings_list" placeholder="Booking ID" />
                <select ref={refs.payMethod} style={{marginTop: '10px'}}>
                    <option value="card">Bank Card</option>
                    <option value="kaspi">Kaspi</option>
                    <option value="cash">Cash</option>
                </select>
                <button onClick={() => request('/api/payments', 'POST', { booking_id: refs.payBkId.current.value, method: refs.payMethod.current.value })}>Create Payment</button>
                <button className="secondary" onClick={() => request('/api/payments')}>List All Payments</button>

                <label style={{marginTop: '16px'}}>Send Push Notification</label>
                <input ref={refs.notifUid} list="users_list" placeholder="User ID" />
                <input ref={refs.notifTitle} placeholder="Message Title" style={{marginTop: '10px'}} />
                <button className="secondary" onClick={() => request('/api/notifications/push', 'POST', { user_id: refs.notifUid.current.value, title: refs.notifTitle.current.value, body: 'Test push from admin panel' })}>Send Push</button>

                <label style={{marginTop: '16px'}}>Send Email (SMTP)</label>
                <input ref={refs.emailTo} placeholder="Recipient email (e.g. user@mail.com)" />
                <input ref={refs.emailUserId} list="users_list" placeholder="User ID (optional)" style={{marginTop: '10px'}} />
                <select ref={refs.emailSubject} style={{marginTop: '10px'}}>
                    <option value="Welcome to Online Parking! Your account has been created.">User Created</option>
                    <option value="Your parking spot has been successfully booked!">🅿Booking Confirmed</option>
                    <option value="Your payment was processed successfully.">Payment Success</option>
                    <option value="Your booking has been cancelled.">Booking Cancelled</option>
                </select>
                <textarea
                    ref={refs.emailBody}
                    placeholder="Email body text..."
                    defaultValue="Hello! This is an automated notification from Online Parking system."
                    style={{marginTop: '10px', minHeight: '60px', width: '100%', boxSizing: 'border-box', padding: '8px', borderRadius: '6px', border: '1px solid #ccc', fontFamily: 'inherit', fontSize: '13px'}}
                />
                <button
                    style={{marginTop: '10px', backgroundColor: '#2563eb', color: '#fff', border: 'none', padding: '10px 18px', borderRadius: '6px', cursor: 'pointer', fontWeight: '600'}}
                    onClick={() => request('/api/notifications/email', 'POST', {
                        to: refs.emailTo.current.value,
                        subject: refs.emailSubject.current.value,
                        body: refs.emailBody.current.value,
                        user_id: refs.emailUserId.current.value || '',
                        type: 'email',
                    })}
                >Send Email via SMTP</button>
            </div>
        );
    };

    return (
        <>
            <header>
                <h1>Online Parking Admin</h1>
                <div className="small">Gateway: <code>/health</code> | API: <code>/api/...</code></div>
            </header>

            <datalist id="users_list">
                {usersCache.map(u => <option key={u.id} value={u.id}>{`${u.first_name} ${u.last_name} — ${u.email}`}</option>)}
            </datalist>
            <datalist id="parkings_list">
                {parkingsCache.map(p => <option key={p.id} value={p.id}>{`${p.id} — ${p.name}`}</option>)}
            </datalist>
            <datalist id="bookings_list">
                {bookingsCache.map(b => <option key={b.id} value={b.id}>{`${b.id} — ${b.status} — ${b.vehicle_plate}`}</option>)}
            </datalist>

            <div className="wrap">
                <HealthCard />
                <AuthCard />
                <AdminUserCard />

                <ParkingCard />
                <SpotsCard />

                <BookingCard />
                <PaymentNotifCard />

                <div className="card full-width">
                    <h2>Output Console</h2>
                    <textarea
                        value={output}
                        readOnly
                        style={{
                            minHeight: '300px',
                            backgroundColor: '#000',
                            color: '#00FF41',
                            fontFamily: 'monospace',
                            fontSize: '13px'
                        }}
                    />
                </div>
            </div>
        </>
    );
}