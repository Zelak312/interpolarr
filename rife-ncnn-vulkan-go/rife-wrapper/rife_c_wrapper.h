#ifndef RIFE_C_WRAPPER_H
#define RIFE_C_WRAPPER_H

#ifdef __cplusplus
extern "C"
{
#endif

    // Opaque pointer to hide C++ implementation
    typedef struct Rife_Ctx Rife_Ctx;

    // Create a new RIFE context
    Rife_Ctx *rife_create(int gpuid, int tta_mode, int tta_temporal_mode, int uhd_mode,
                          int num_threads, int rife_v2, int rife_v4, int padding);

    // Destroy RIFE context
    void rife_destroy(Rife_Ctx *ctx);

    // Load model
    int rife_load(Rife_Ctx *ctx, const char *modeldir);

    // Process frames directly with buffers
    int rife_process_frames(Rife_Ctx *ctx,
                            unsigned char *data0, unsigned char *data1,
                            int width, int height, int elempack,
                            unsigned char *out_data, float timestep);

    // Get GPU count
    int rife_get_gpu_count(void);

#ifdef __cplusplus
}
#endif

#endif // RIFE_C_WRAPPER_H