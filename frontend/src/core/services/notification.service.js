// Notification Service
export function requestNotificationPermission() {
    if ("Notification" in window && Notification.permission === "default") {
        Notification.requestPermission()
            .then(permission => {
                console.log("Notification permission:", permission);
            })
            .catch(err => {
                console.error("Notification error:", err);
            });
    }
}

export function sendNotification(title, Message) {
    if ("Notification" in window && Notification.permission === "granted") {
        new Notification(title, { 
            body: Message, 
            icon: 'https://cdn-icons-png.flaticon.com/512/564/564619.png' 
        });
    }
}
