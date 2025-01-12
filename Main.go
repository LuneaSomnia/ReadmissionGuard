package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "github.com/dgraph-io/dgo/v2"
    "github.com/dgraph-io/dgo/v2/protos/api"
    "github.com/modusapi/modus-go"
    "google.golang.org/grpc"
)

type Server struct {
    modus *modus.Client
    dgraph *dgo.Dgraph
}

type PatientData struct {
    ID string `json:"id"`
    Age int `json:"age"`
    PreviousAdmissions int `json:"previousAdmissions"`
    ChronicConditions []string `json:"chronicConditions"`
    Medications []string `json:"medications"`
}

type HistoricalRecord struct {
    Name string `json:"name"`
    Age int `json:"age"`
    Admissions []Admission `json:"admissions"`
    Medications []Medication `json:"medications"`
}

type Admission struct {
    Date string `json:"date"`
    Diagnosis string `json:"diagnosis"`
    Treatment string `json:"treatment"`
}

type Medication struct {
    Name string `json:"name"`
    Dosage string `json:"dosage"`
}

func main() {
    server := NewServer()
    http.HandleFunc("/api/patient", server.HandlePatient)
    http.HandleFunc("/api/risk", server.HandleRiskAssessment)
    http.HandleFunc("/api/interventions", server.HandleInterventions)
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func NewServer() *Server {
    modusClient := modus.NewClient()
    dgraphClient := connectToDgraph()
    return &Server{
        modus: modusClient,
        dgraph: dgraphClient,
    }
}

func connectToDgraph() *dgo.Dgraph {
    conn, err := grpc.Dial("localhost:9080", grpc.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    return dgo.NewDgraphClient(api.NewDgraphClient(conn))
}

func (s *Server) HandlePatient(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        var patient PatientData
        if err := json.NewDecoder(r.Body).Decode(&patient); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        if err := s.storePatientData(patient); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusCreated)
    case http.MethodGet:
        patientID := r.URL.Query().Get("id")
        history, err := s.queryPatientHistory(patientID)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        json.NewEncoder(w).Encode(history)
    }
}

func (s *Server) HandleRiskAssessment(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var patientData PatientData
    if err := json.NewDecoder(r.Body).Decode(&patientData); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    prediction, err := s.predictReadmissionRisk(patientData)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(prediction)
}

func (s *Server) predictReadmissionRisk(data PatientData) (float64, error) {
    ctx := context.Background()
    response, err := s.modus.PredictReadmissionRisk(ctx, data)
    if err != nil {
        return 0, err
    }
    return response.RiskScore, nil
}

func (s *Server) HandleInterventions(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    patientID := r.URL.Query().Get("id")
    history, err := s.queryPatientHistory(patientID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    interventions := generateInterventions(history)
    json.NewEncoder(w).Encode(interventions)
}

func (s *Server) storePatientData(patient PatientData) error {
    ctx := context.Background()
    txn := s.dgraph.NewTxn()
    defer txn.Discard(ctx)

    mutation := &api.Mutation{
        SetNquads: []byte(`
            _:patient <name> "` + patient.ID + `" .
            _:patient <age> "` + string(patient.Age) + `" .
            _:patient <condition> "` + patient.ChronicConditions[0] + `" .
        `),
    }

    _, err := txn.Mutate(ctx, mutation)
    if err != nil {
        return err
    }

    return txn.Commit(ctx)
}

func (s *Server) queryPatientHistory(patientID string) ([]HistoricalRecord, error) {
    ctx := context.Background()
    query := `
        {
            patient(func: eq(name, "` + patientID + `")) {
                name
                age
                admissions {
                    date
                    diagnosis
                    treatment
                }
                medications {
                    name
                    dosage
                }
            }
        }
    `

    resp, err := s.dgraph.NewTxn().Query(ctx, query)
    if err != nil {
        return nil, err
    }

    var result struct {
        Patient []HistoricalRecord `json:"patient"`
    }
    
    if err := json.Unmarshal(resp.Json, &result); err != nil {
        return nil, err
    }

    return result.Patient, nil
}

func generateInterventions(history []HistoricalRecord) []string {
    // Implementation of intervention generation based on patient history
    interventions := []string{}
    // Add intervention logic here
    return interventions
}
